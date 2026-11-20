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
