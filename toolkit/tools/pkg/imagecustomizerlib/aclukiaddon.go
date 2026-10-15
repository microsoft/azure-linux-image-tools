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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

var (
	ErrAclUkiAddonTemplates = NewImageCustomizerError("AclUkiAddon:Templates",
		"failed to update the root hash in the ACL UKI addon templates")
)

const (
	// ACL's addon templates on the ESP, including the A/B slot addons that Trident activates.
	aclUkiAddonTemplatesDir = "acl/uki-addons"

	// The kernel argument that holds the /usr dm-verity root hash.
	aclUsrHashArgName = "usrhash"
)

// aclUpdateSlotAddonTemplates gives the rebuilt UKIs' /usr root hash to the addon templates that carry one: the A/B
// slot addons, which Trident copies into a UKI's addon directory to switch slots. The active slot's template becomes a
// copy of its rebuilt addon.
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

// aclPlanSlotAddonTemplate decides how a slot addon template gets usrHash: copied from the rebuilt UKIs' same-named
// addons, which must match it apart from the root hash, or rebuilt with newCmdline if no UKI has that addon.
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
			return "", false, false, fmt.Errorf("A/B slot addon args can't change apart from the root hash "+
				"(addon='%s', template='%s')", activeAddonCmdline, newCmdline)
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
	args, err := parseKernelCmdlineArgs(cmdline)
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
	args, err := parseKernelCmdlineArgs(cmdline)
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
