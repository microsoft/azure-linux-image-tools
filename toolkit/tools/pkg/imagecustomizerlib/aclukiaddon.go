// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

var (
	ErrAclUkiAddonSplit = NewImageCustomizerError("AclUkiAddon:Split",
		"failed to split kernel command line into ACL UKI addons")
	ErrAclUkiAddonEmptyPersistentCmdline = NewImageCustomizerError("AclUkiAddon:EmptyPersistentCmdline",
		"kernel command line has no persistent arguments")
)

const (
	// The kernel argument that makes ACL run its first-boot provisioning.
	aclFirstBootArg = "flatcar.first_boot=detected"

	// Name of the transient addon that carries only aclFirstBootArg.
	aclFirstBootAddonName = "firstboot.addon.efi"

	// Name of the addon that carries the platform arguments (flatcar.oem.id and console settings), built per image
	// format by the ACL image build.
	aclOemAddonName = "oem.addon.efi"
)

// aclKeptAddonNames lists the addons the ACL image ships that create mode keeps as separate addons, rebuilt with
// their own command line, instead of folding them into the persistent addon. The first-boot addon is transient and
// handled separately.
var aclKeptAddonNames = []string{aclOemAddonName}

// aclGetUkiAddonSpecs returns ACL's cmdline addon layout: a persistent <kernel>.addon.efi holding every argument that
// belongs to neither a kept addon nor aclFirstBootArg, the kept addons (see aclKeptAddonNames) with their own command
// lines, plus a transient firstboot.addon.efi holding exactly aclFirstBootArg. If the command line does not contain
// aclFirstBootArg, the image has already completed its first boot and no first-boot addon spec is returned.
func aclGetUkiAddonSpecs(kernel string, cmdline string, baseAddons map[string]string) ([]UkiAddonSpec, error) {
	keptAddons := []UkiAddonSpec(nil)
	for _, name := range aclKeptAddonNames {
		addonCmdline, ok := baseAddons[name]
		if !ok {
			continue
		}
		if addonCmdline == "" {
			return nil, fmt.Errorf("%w:\nbase image addon (%s) has an empty command line", ErrAclUkiAddonSplit, name)
		}

		keptAddons = append(keptAddons, UkiAddonSpec{FileName: name, Cmdline: addonCmdline})
	}

	persistentCmdline, hasFirstBootArg, err := aclSplitCmdline(cmdline, keptAddons)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	if persistentCmdline == "" {
		return nil, fmt.Errorf("%w (kernel='%s', cmdline='%s')", ErrAclUkiAddonEmptyPersistentCmdline, kernel, cmdline)
	}

	specs := []UkiAddonSpec{
		{FileName: ukiAddonFileName(kernel), Cmdline: persistentCmdline},
	}
	specs = append(specs, keptAddons...)

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

// aclSplitCmdline removes every occurrence of aclFirstBootArg and one occurrence of each kept addon's arguments from
// cmdline, returning the remaining arguments and whether aclFirstBootArg was present.
func aclSplitCmdline(cmdline string, keptAddons []UkiAddonSpec) (string, bool, error) {
	args, err := aclParseCmdlineArgs(cmdline)
	if err != nil {
		return "", false, err
	}

	// Count each kept addon's arguments so that exactly the arguments the addon contributes are removed from the
	// merged command line, and an identical argument added by the user stays.
	keptArgCounts := map[string]int{}
	for _, addon := range keptAddons {
		addonArgs, err := aclParseCmdlineArgs(addon.Cmdline)
		if err != nil {
			return "", false, fmt.Errorf("failed to parse addon (%s) command line:\n%w", addon.FileName, err)
		}

		for _, arg := range addonArgs {
			keptArgCounts[arg]++
		}
	}

	hasFirstBootArg := false
	persistentArgs := []string(nil)
	for _, arg := range args {
		if arg == aclFirstBootArg {
			hasFirstBootArg = true
			continue
		}

		if keptArgCounts[arg] > 0 {
			keptArgCounts[arg]--
			continue
		}

		persistentArgs = append(persistentArgs, arg)
	}

	return GrubArgsToString(persistentArgs), hasFirstBootArg, nil
}

// aclParseCmdlineArgs splits a kernel command line into its arguments.
func aclParseCmdlineArgs(cmdline string) ([]string, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	argStrings := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			// The parsed form of an arg with a variable expansion is truncated at the expansion, so the arg cannot
			// be rewritten faithfully.
			return nil, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}

		argStrings = append(argStrings, arg.Arg)
	}

	return argStrings, nil
}
