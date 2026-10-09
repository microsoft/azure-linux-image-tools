// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

var (
	ErrAclUkiAddonSplit = NewImageCustomizerError("AclUkiAddon:Split",
		"failed to split kernel command line across the ACL UKI and its addons")
	ErrAclUkiAddonEmptyPersistentCmdline = NewImageCustomizerError("AclUkiAddon:EmptyPersistentCmdline",
		"kernel command line has no persistent arguments")
	ErrAclUkiAddonAmbiguousArg = NewImageCustomizerError("AclUkiAddon:AmbiguousArg",
		"changed kernel command-line arg is in more than one file of the base image's UKI")
	ErrAclUkiAddonTemplates = NewImageCustomizerError("AclUkiAddon:Templates",
		"failed to update the root hash in the ACL UKI addon templates")
)

const (
	// The kernel argument that makes ACL run its first-boot provisioning.
	aclFirstBootArg = "flatcar.first_boot=detected"

	// Name of the transient addon that carries only aclFirstBootArg.
	aclFirstBootAddonName = "firstboot.addon.efi"

	// ACL's addon templates on the ESP, including the A/B slot addons that Trident activates.
	aclUkiAddonTemplatesDir = "acl/uki-addons"

	// The kernel argument that holds the /usr dm-verity root hash.
	aclUsrHashArgName = "usrhash"
)

// aclBaseArg is an argument of the base image's UKI command line, in the main UKI or in one of its addons.
type aclBaseArg struct {
	arg grubConfigLinuxArg
	// The argument of the new command line that takes this argument's place, if any.
	newArg string
	used   bool
}

// aclGetUkiLayout splits the command line of an ACL UKI the way the base image's UKI did: an argument identical to a
// base argument stays where that argument was, an argument that changes the value of a base argument takes its
// place, and a new argument goes into the main UKI, where the UKI's signature covers it. A new argument with the name
// of an addon's argument goes after it instead, at the end of the last file that has that name, so that it still
// overrides the base value. aclFirstBootArg always goes into the transient firstboot.addon.efi. An addon left with no
// arguments is dropped. The other files of the base image's addon directory are kept.
func aclGetUkiLayout(cmdline string, baseLayout *UkiLayout) (UkiLayout, error) {
	args, err := aclParseCmdlineArgs(cmdline)
	if err != nil {
		return UkiLayout{}, fmt.Errorf("%w:\n%w", ErrAclUkiAddonSplit, err)
	}

	base := UkiLayout{}
	if baseLayout != nil {
		base = *baseLayout
	}

	// The main UKI first, then the addons by file name: the order in which systemd-stub concatenates them.
	fileNames := append([]string{""}, slices.Sorted(maps.Keys(base.Addons))...)
	baseArgs := make([][]aclBaseArg, len(fileNames))
	for i, fileName := range fileNames {
		fileCmdline := base.MainCmdline
		if fileName != "" {
			fileCmdline = base.Addons[fileName]
			if fileCmdline == "" {
				// An addon without a command line carries something else (e.g. a devicetree).
				return UkiLayout{}, fmt.Errorf("%w:\nbase image addon (%s) has no kernel command line",
					ErrAclUkiAddonSplit, fileName)
			}
		}

		fileArgs, err := aclParseCmdlineArgs(fileCmdline)
		if err != nil {
			return UkiLayout{}, fmt.Errorf("%w (file='%s'):\n%w", ErrAclUkiAddonSplit, fileName, err)
		}

		for _, arg := range fileArgs {
			if arg.Arg != aclFirstBootArg {
				baseArgs[i] = append(baseArgs[i], aclBaseArg{arg: arg})
			}
		}
	}

	hasFirstBootArg := false
	placed := make([]bool, len(args))
	for i, arg := range args {
		if arg.Arg == aclFirstBootArg {
			hasFirstBootArg = true
			placed[i] = true
			continue
		}

		placed[i] = aclPlaceArg(baseArgs, arg, func(baseArg grubConfigLinuxArg) bool {
			return baseArg.Arg == arg.Arg
		})
	}

	newArgs := make([][]string, len(fileNames))
	for i, arg := range args {
		if placed[i] {
			continue
		}

		fileIndexes := map[int]bool{}
		lastFileIndex := 0
		for fileIndex := range baseArgs {
			for _, baseArg := range baseArgs[fileIndex] {
				if baseArg.arg.Name != arg.Name {
					continue
				}

				lastFileIndex = fileIndex
				if !baseArg.used {
					fileIndexes[fileIndex] = true
				}
			}
		}
		if len(fileIndexes) > 1 {
			return UkiLayout{}, fmt.Errorf("%w (arg='%s')", ErrAclUkiAddonAmbiguousArg, arg.Arg)
		}

		if !aclPlaceArg(baseArgs, arg, func(baseArg grubConfigLinuxArg) bool { return baseArg.Name == arg.Name }) {
			newArgs[lastFileIndex] = append(newArgs[lastFileIndex], arg.Arg)
		}
	}

	layout := UkiLayout{
		Addons:     map[string]string{},
		ExtraFiles: maps.Clone(base.ExtraFiles),
	}
	for i, fileName := range fileNames {
		fileArgs := []string(nil)
		for _, baseArg := range baseArgs[i] {
			if baseArg.used {
				fileArgs = append(fileArgs, baseArg.newArg)
			}
		}
		fileArgs = append(fileArgs, newArgs[i]...)

		switch {
		case fileName == "":
			layout.MainCmdline = GrubArgsToString(fileArgs)

		case len(fileArgs) == 0:
			if fileName != aclFirstBootAddonName {
				logger.Log.Infof("UKI addon (%s) has no kernel command-line args left; not adding it", fileName)
			}

		default:
			layout.Addons[fileName] = GrubArgsToString(fileArgs)
		}
	}

	if layout.MainCmdline == "" && len(layout.Addons) == 0 {
		return UkiLayout{}, fmt.Errorf("%w (cmdline='%s')", ErrAclUkiAddonEmptyPersistentCmdline, cmdline)
	}

	if hasFirstBootArg {
		layout.Addons[aclFirstBootAddonName] = strings.TrimSpace(layout.Addons[aclFirstBootAddonName] + " " +
			aclFirstBootArg)
	} else {
		logger.Log.Infof("Kernel command line has no (%s); not adding a first-boot addon", aclFirstBootArg)
	}

	return layout, nil
}

// aclPlaceArg gives arg the place of the first unused base arg that matches, in command-line order, and reports
// whether it found one.
func aclPlaceArg(baseArgs [][]aclBaseArg, arg grubConfigLinuxArg, matches func(grubConfigLinuxArg) bool) bool {
	for fileIndex := range baseArgs {
		for argIndex := range baseArgs[fileIndex] {
			baseArg := &baseArgs[fileIndex][argIndex]
			if !baseArg.used && matches(baseArg.arg) {
				baseArg.used = true
				baseArg.newArg = arg.Arg
				return true
			}
		}
	}

	return false
}

// aclParseCmdlineArgs splits a kernel command line into its arguments.
func aclParseCmdlineArgs(cmdline string) ([]grubConfigLinuxArg, error) {
	tokens, err := grub.TokenizeConfig(cmdline)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize kernel command line:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel command-line args:\n%w", err)
	}

	for _, arg := range args {
		if arg.ValueHasVarExpansion {
			// The parsed form of an arg with a variable expansion is truncated at the expansion, so the arg cannot
			// be rewritten faithfully.
			return nil, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}
	}

	return args, nil
}

// aclUpdateSlotAddonTemplates gives the rebuilt UKIs' /usr root hash to the addon templates that carry a root hash:
// the A/B slot addons, which Trident copies into the UKI's addon directory when it switches slots. A template with a
// same-named addon in the UKI's addon directory gets that addon's bytes, so that both copies stay identical; any other
// template is rebuilt with the new root hash. Templates without a root hash are left alone.
func aclUpdateSlotAddonTemplates(espDir string, addonStubPath string, kernelInfo map[string]UkiKernelInfo,
	buildDir string,
) error {
	templateDir := filepath.Join(espDir, aclUkiAddonTemplatesDir)
	entries, err := os.ReadDir(templateDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w:\nfailed to read directory (%s):\n%w", ErrAclUkiAddonTemplates, templateDir, err)
	}

	usrHash, err := aclGetUsrHash(kernelInfo)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclUkiAddonTemplates, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".addon.efi") {
			continue
		}

		err := aclUpdateSlotAddonTemplate(espDir, filepath.Join(templateDir, entry.Name()), usrHash, addonStubPath,
			kernelInfo, buildDir)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrAclUkiAddonTemplates, err)
		}
	}

	return nil
}

func aclUpdateSlotAddonTemplate(espDir string, templatePath string, usrHash string, addonStubPath string,
	kernelInfo map[string]UkiKernelInfo, buildDir string,
) error {
	templateCmdline, err := extractCmdlineFromSinglePE(templatePath, buildDir)
	if err != nil {
		return fmt.Errorf("failed to read addon template (%s):\n%w", templatePath, err)
	}

	// The template build writes the command line with a trailing newline.
	templateCmdline = strings.TrimSpace(templateCmdline)
	if !strings.Contains(" "+templateCmdline, " "+aclUsrHashArgName+"=") {
		return nil
	}

	activeAddonPaths := []string(nil)
	activeAddonCmdlines := []string(nil)
	for _, kernel := range slices.Sorted(maps.Keys(kernelInfo)) {
		addonPath := filepath.Join(espDir, UkiOutputDir, kernel+".efi.extra.d", filepath.Base(templatePath))
		exists, err := file.PathExists(addonPath)
		if err != nil {
			return fmt.Errorf("failed to check for addon (%s):\n%w", addonPath, err)
		}
		if !exists {
			continue
		}

		addonCmdline, err := extractCmdlineFromSinglePE(addonPath, buildDir)
		if err != nil {
			return fmt.Errorf("failed to read addon (%s):\n%w", addonPath, err)
		}

		activeAddonPaths = append(activeAddonPaths, addonPath)
		activeAddonCmdlines = append(activeAddonCmdlines, strings.TrimSpace(addonCmdline))
	}

	newCmdline, copyActiveAddon, rebuild, err := aclPlanSlotAddonTemplate(templateCmdline, activeAddonCmdlines,
		usrHash)
	if err != nil {
		return fmt.Errorf("addon template (%s):\n%w", templatePath, err)
	}

	switch {
	case copyActiveAddon:
		err = file.Copy(activeAddonPaths[0], templatePath)
		if err != nil {
			return fmt.Errorf("failed to copy addon (%s) to its template (%s):\n%w", activeAddonPaths[0], templatePath,
				err)
		}

		logger.Log.Infof("Updated UKI addon template from the rebuilt addon: (%s)", templatePath)

	case rebuild:
		err = buildUkiAddonFile(templatePath, newCmdline, addonStubPath)
		if err != nil {
			return fmt.Errorf("failed to rebuild addon template (%s):\n%w", templatePath, err)
		}

		logger.Log.Infof("Rebuilt UKI addon template with the new root hash: (%s)", templatePath)
	}

	return nil
}

// aclPlanSlotAddonTemplate decides how an addon template that carries a root hash gets usrHash: from the same-named
// addons of the rebuilt UKIs (copyActiveAddon), whose command lines must all be the template's with the new root hash,
// or by rebuilding it with newCmdline (rebuild) when no UKI has that addon and the root hash changed.
func aclPlanSlotAddonTemplate(templateCmdline string, activeAddonCmdlines []string, usrHash string,
) (newCmdline string, copyActiveAddon bool, rebuild bool, err error) {
	if usrHash == "" {
		return "", false, false, fmt.Errorf("the rebuilt UKIs have no (%s) arg", aclUsrHashArgName)
	}

	newCmdline, changed, err := aclSetArgValue(templateCmdline, aclUsrHashArgName, usrHash)
	if err != nil {
		return "", false, false, err
	}

	for _, activeAddonCmdline := range activeAddonCmdlines {
		if activeAddonCmdline != newCmdline {
			return "", false, false, fmt.Errorf("rebuilt A/B slot addon differs from its template in more than "+
				"the root hash; customization can't change the args of a slot addon (addon='%s', template='%s')",
				activeAddonCmdline, newCmdline)
		}
	}

	if len(activeAddonCmdlines) > 0 {
		return newCmdline, true, false, nil
	}

	return newCmdline, false, changed, nil
}

// aclGetUsrHash returns the /usr root hash of the rebuilt UKIs, which must all agree, or "" if they have none.
func aclGetUsrHash(kernelInfo map[string]UkiKernelInfo) (string, error) {
	hashes := map[string]bool{}
	for kernel, info := range kernelInfo {
		hash, err := aclGetArgValue(info.Cmdline, aclUsrHashArgName)
		if err != nil {
			return "", fmt.Errorf("failed to read the root hash of kernel (%s):\n%w", kernel, err)
		}

		hashes[hash] = true
	}

	if len(hashes) > 1 {
		return "", fmt.Errorf("UKIs have different /usr root hashes (%v)", slices.Sorted(maps.Keys(hashes)))
	}

	for hash := range hashes {
		return hash, nil
	}

	return "", nil
}

// aclGetArgValue returns the value of the named arg of a kernel command line, or "" if the arg is absent.
func aclGetArgValue(cmdline string, name string) (string, error) {
	args, err := aclParseCmdlineArgs(cmdline)
	if err != nil {
		return "", err
	}

	value := ""
	found := false
	for _, arg := range args {
		if arg.Name != name {
			continue
		}

		if found {
			return "", fmt.Errorf("kernel command line has more than one (%s) arg", name)
		}

		value = arg.Value
		found = true
	}

	return value, nil
}

// aclSetArgValue sets the value of every occurrence of the named arg of a kernel command line, and reports whether
// that changed any.
func aclSetArgValue(cmdline string, name string, value string) (string, bool, error) {
	args, err := aclParseCmdlineArgs(cmdline)
	if err != nil {
		return "", false, err
	}

	changed := false
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.Name != name {
			newArgs = append(newArgs, arg.Arg)
			continue
		}

		newArg := name + "=" + value
		if arg.Arg != newArg {
			changed = true
		}
		newArgs = append(newArgs, newArg)
	}

	return GrubArgsToString(newArgs), changed, nil
}
