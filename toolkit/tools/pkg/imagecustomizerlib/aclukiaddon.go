// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

var (
	ErrAclUkiAddonSplit = NewImageCustomizerError("AclUkiAddon:Split",
		"failed to split kernel command line into ACL UKI addons")
	ErrAclUkiAddonEmptyPersistentCmdline = NewImageCustomizerError("AclUkiAddon:EmptyPersistentCmdline",
		"kernel command line has no persistent arguments")
	ErrAclOemIdInMainUki = NewImageCustomizerError("AclOemId:InMainUki",
		"main UKI's own command line sets the OEM id, which 'uki: mode: modify' cannot rewrite")
)

const (
	// The kernel argument that makes ACL run its first-boot provisioning.
	aclFirstBootArg = "flatcar.first_boot=detected"

	// Name of the transient addon that carries only aclFirstBootArg.
	aclFirstBootAddonName = "firstboot.addon.efi"
)

// aclGetUkiAddonSpecs returns ACL's cmdline addon layout: a persistent <kernel>.addon.efi holding
// every argument except aclFirstBootArg, plus a transient firstboot.addon.efi holding exactly that
// argument. If the command line does not contain the argument, the image has already completed its
// first boot and only the persistent addon spec is returned.
func aclGetUkiAddonSpecs(kernel string, cmdline string) ([]UkiAddonSpec, error) {
	persistentCmdline, hasFirstBootArg, err := aclStripFirstBootArg(cmdline)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	if persistentCmdline == "" {
		return nil, fmt.Errorf("%w (kernel='%s', cmdline='%s')", ErrAclUkiAddonEmptyPersistentCmdline, kernel, cmdline)
	}

	specs := []UkiAddonSpec{
		{FileName: ukiAddonFileName(kernel), Cmdline: persistentCmdline},
	}

	if !hasFirstBootArg {
		logger.Log.Infof("Kernel (%s) command line has no (%s); not adding a first-boot addon", kernel,
			aclFirstBootArg)
		return specs, nil
	}

	specs = append(specs, UkiAddonSpec{
		FileName: aclFirstBootAddonName,
		Cmdline:  aclFirstBootArg,
	})
	return specs, nil
}

// aclStripFirstBootArg removes every occurrence of aclFirstBootArg from cmdline and reports
// whether any was present.
func aclStripFirstBootArg(cmdline string) (string, bool, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return "", false, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	hasFirstBootArg := false
	persistentArgs := []string(nil)
	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			// The parsed form of an arg with a variable expansion is truncated at the expansion, so
			// the arg cannot be rewritten faithfully.
			return "", false, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}

		if arg.Arg == aclFirstBootArg {
			hasFirstBootArg = true
			continue
		}

		persistentArgs = append(persistentArgs, arg.Arg)
	}

	return GrubArgsToString(persistentArgs), hasFirstBootArg, nil
}

// aclClearOemIdOutsideIcAddon removes stale OEM id tokens from everything on the ESP that
// contributes to a UKI's kernel command line except the IC-managed addon, which the caller rewrites
// itself.
//
// Under 'uki: mode: modify' IC only rewrites <kernel>.addon.efi, but ACL's base ESP also ships
// other addons (e.g. an oem addon). systemd-stub concatenates the main UKI's .cmdline with every
// addon's, so appending the new id is not sufficient: a stale flatcar.oem.id=<base> token left in a
// sibling addon still satisfies presence-based ConditionKernelCommandLine matches (the azure
// metadata/hostname agent), which is the exact failure acl.oemId exists to fix. Last-occurrence-wins
// only settles the platform id, not those Condition matches.
//
// A sibling addon that carries nothing but OEM id tokens is deleted; otherwise it is rebuilt without
// them. The main UKI is signed and deliberately preserved in modify mode, so an OEM id there is a
// hard error rather than something to silently ignore.
func aclClearOemIdOutsideIcAddon(ukiFilePath string, icAddonFileName string, stubPath string,
	buildDir string,
) error {
	_, mainHasOemId, err := aclReadOemIdArgs(ukiFilePath, buildDir)
	if err != nil {
		return fmt.Errorf("failed to inspect command line of UKI (%s):\n%w", filepath.Base(ukiFilePath), err)
	}

	if mainHasOemId {
		return fmt.Errorf("%w (uki='%s'); use 'uki: mode: create' to set 'acl.oemId' on this image",
			ErrAclOemIdInMainUki, filepath.Base(ukiFilePath))
	}

	ukiFileName := filepath.Base(ukiFilePath)
	addonDirPath := filepath.Join(filepath.Dir(ukiFilePath), fmt.Sprintf("%s.extra.d", ukiFileName))

	entries, err := os.ReadDir(addonDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read UKI addon directory (%s):\n%w", addonDirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".addon.efi") || entry.Name() == icAddonFileName {
			continue
		}

		addonPath := filepath.Join(addonDirPath, entry.Name())

		remainingArgs, hasOemId, err := aclReadOemIdArgs(addonPath, buildDir)
		if err != nil {
			return fmt.Errorf("failed to inspect command line of UKI addon (%s):\n%w", entry.Name(), err)
		}

		if !hasOemId {
			continue
		}

		if len(remainingArgs) == 0 {
			logger.Log.Infof("Removing UKI addon (%s): it only set the OEM id, which 'acl.oemId' replaces",
				entry.Name())
			err = os.Remove(addonPath)
			if err != nil {
				return fmt.Errorf("failed to remove UKI addon (%s):\n%w", entry.Name(), err)
			}
			continue
		}

		logger.Log.Infof("Rebuilding UKI addon (%s) without its OEM id, which 'acl.oemId' replaces",
			entry.Name())

		ukifyCmd := []string{
			"build",
			fmt.Sprintf("--cmdline=%s", GrubArgsToString(remainingArgs)),
			fmt.Sprintf("--stub=%s", stubPath),
			fmt.Sprintf("--output=%s", addonPath),
		}

		err = shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
		if err != nil {
			return fmt.Errorf("failed to rebuild UKI addon (%s) without its OEM id:\n%w", entry.Name(), err)
		}
	}

	return nil
}

// aclReadOemIdArgs reads a PE image's kernel command line and splits out its OEM id tokens,
// returning the remaining args and whether any OEM id was present.
//
// A PE with no .cmdline section contributes nothing to the kernel command line, so it carries no
// OEM id: ACL's main UKI keeps its command line entirely in addons, and ukify omits the section
// altogether when the command line is empty.
func aclReadOemIdArgs(pePath string, buildDir string) ([]string, bool, error) {
	cmdline, err := extractCmdlineFromSinglePEIfPresent(pePath, buildDir)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read kernel command line:\n%w", err)
	}

	if cmdline == "" {
		return nil, false, nil
	}

	return stripAclOemIdArgs(cmdline)
}
