// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// aclVerityAddonName is the addon file name Trident swaps to switch the active dm-verity slot
// during an A/B update. It must match Trident's expected file name exactly.
const aclVerityAddonName = "verity.addon.efi"

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
)

// aclGetUkiAddonSpecs returns ACL's cmdline addon layout: a persistent <kernel>.addon.efi holding
// every argument except aclFirstBootArg and dm-verity args, a verity.addon.efi holding the current
// dm-verity slot's arguments (so Trident can swap it independently during an A/B update), and a
// transient firstboot.addon.efi holding exactly aclFirstBootArg. Either of the latter two addon
// specs is omitted if its arguments are not present in cmdline (e.g. verity disabled, or the image
// has already completed its first boot).
func aclGetUkiAddonSpecs(kernel string, cmdline string) ([]UkiAddonSpec, error) {
	afterFirstBoot, hasFirstBootArg, err := aclStripFirstBootArg(cmdline)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	persistentCmdline, verityCmdline, err := aclStripVerityArgs(afterFirstBoot)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	if persistentCmdline == "" {
		return nil, fmt.Errorf("%w (kernel='%s', cmdline='%s')", ErrAclUkiAddonEmptyPersistentCmdline, kernel, cmdline)
	}

	specs := []UkiAddonSpec{
		{FileName: ukiAddonFileName(kernel), Cmdline: persistentCmdline},
	}

	if verityCmdline == "" {
		logger.Log.Infof("Kernel (%s) command line has no dm-verity arguments; not adding a verity addon", kernel)
	} else {
		specs = append(specs, UkiAddonSpec{
			FileName: aclVerityAddonName,
			Cmdline:  verityCmdline,
		})
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

// aclStripVerityArgs removes every dm-verity kernel argument (see verityArgPrefixes in
// customizeuki.go) from cmdline and returns the remaining ("persistent") cmdline together with the
// removed verity arguments, in their original relative order, so they can be routed into ACL's
// per-slot verity.addon.efi instead of being baked permanently into the persistent addon.
func aclStripVerityArgs(cmdline string) (string, string, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return "", "", fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	persistentArgs := []string(nil)
	verityArgs := []string(nil)
	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			// The parsed form of an arg with a variable expansion is truncated at the expansion, so
			// the arg cannot be rewritten faithfully.
			return "", "", fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}

		if isVerityArg(arg.Arg) {
			verityArgs = append(verityArgs, arg.Arg)
			continue
		}

		persistentArgs = append(persistentArgs, arg.Arg)
	}

	return GrubArgsToString(persistentArgs), GrubArgsToString(verityArgs), nil
}
