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

const (
	// Copies of the base image's non-addon files in UKI addon directories, under UkiBuildDir, by kernel.
	ukiExtraFilesDir = "extra-files"
)

// UkiLayout describes how a UKI's kernel command line is split across the main UKI and the addons in its
// <uki>.efi.extra.d/ directory, and which other files that directory holds.
type UkiLayout struct {
	MainCmdline string `json:"mainCmdline,omitempty"`
	// Addon file name -> addon command line.
	Addons map[string]string `json:"addons,omitempty"`
	// Name of another file in the addon directory (e.g. a credential) -> its SHA-256, to tell if it changed.
	ExtraFiles map[string]string `json:"extraFiles,omitempty"`
}

// ukiBaseArg is an argument of the base image's UKI command line, in the main UKI or in one of its addons.
type ukiBaseArg struct {
	arg grubConfigLinuxArg
	// The argument of the new command line that takes this argument's place, if any.
	newArg string
	used   bool
}

// readUkiBaseLayouts reads the kernel command line and the layout of each UKI, and saves a copy of the other files in
// each UKI's addon directory so that they can be put back with the rebuilt UKI.
func readUkiBaseLayouts(espDir string, buildDir string) (map[string]UkiKernelInfo, error) {
	ukiFiles, err := getUkiFiles(espDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get UKI files:\n%w", err)
	}

	kernelInfo := make(map[string]UkiKernelInfo, len(ukiFiles))
	for _, ukiFile := range ukiFiles {
		kernel, err := getKernelNameFromUki(ukiFile)
		if err != nil {
			return nil, fmt.Errorf("failed to extract kernel name from UKI file (%s):\n%w", ukiFile, err)
		}

		layout, err := readUkiLayout(ukiFile, buildDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read the layout of UKI (%s):\n%w", ukiFile, err)
		}

		err = saveUkiExtraFiles(ukiFile, kernel, layout, buildDir)
		if err != nil {
			return nil, err
		}

		cmdline, err := mergeUkiCmdlineParts(layout.MainCmdline, layout.Addons)
		if err != nil {
			return nil, fmt.Errorf("failed to extract base kernel command-line from UKI (%s):\n%w", ukiFile, err)
		}

		kernelInfo[kernel] = UkiKernelInfo{Cmdline: cmdline, BaseLayout: &layout}
	}

	return kernelInfo, nil
}

// readUkiLayout reads how a UKI's command line is split across the main UKI and its addons, and the SHA-256 of each
// other file in its addon directory.
func readUkiLayout(ukiFile string, buildDir string) (UkiLayout, error) {
	mainCmdline, addonCmdlines, err := extractUkiCmdlineParts(ukiFile, buildDir)
	if err != nil {
		return UkiLayout{}, err
	}

	layout := UkiLayout{MainCmdline: strings.TrimSpace(mainCmdline), Addons: addonCmdlines}

	addonDir := ukiFile + ".extra.d"
	entries, err := os.ReadDir(addonDir)
	if errors.Is(err, fs.ErrNotExist) {
		return layout, nil
	}
	if err != nil {
		return UkiLayout{}, fmt.Errorf("failed to read UKI addon directory (%s):\n%w", addonDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".addon.efi") {
			continue
		}

		filePath := filepath.Join(addonDir, entry.Name())
		hash, err := file.GenerateSHA256(filePath)
		if err != nil {
			return UkiLayout{}, fmt.Errorf("failed to hash UKI addon directory file (%s):\n%w", filePath, err)
		}

		if layout.ExtraFiles == nil {
			layout.ExtraFiles = map[string]string{}
		}
		layout.ExtraFiles[entry.Name()] = hash
	}

	return layout, nil
}

// saveUkiExtraFiles copies the other files of a UKI's addon directory into the build dir, under the kernel's name.
func saveUkiExtraFiles(ukiFile string, kernel string, layout UkiLayout, buildDir string) error {
	for _, fileName := range slices.Sorted(maps.Keys(layout.ExtraFiles)) {
		filePath := filepath.Join(ukiFile+".extra.d", fileName)
		err := file.Copy(filePath, ukiExtraFileSavePath(buildDir, kernel, fileName))
		if err != nil {
			return fmt.Errorf("failed to save UKI addon directory file (%s):\n%w", filePath, err)
		}
	}

	return nil
}

// copyUkiExtraFiles gives a kernel a copy of the files saved by saveUkiExtraFiles for another kernel.
func copyUkiExtraFiles(buildDir string, fromKernel string, toKernel string, layout UkiLayout) error {
	for _, fileName := range slices.Sorted(maps.Keys(layout.ExtraFiles)) {
		err := file.Copy(ukiExtraFileSavePath(buildDir, fromKernel, fileName),
			ukiExtraFileSavePath(buildDir, toKernel, fileName))
		if err != nil {
			return fmt.Errorf("failed to copy UKI addon directory file (%s) for kernel (%s):\n%w", fileName, toKernel,
				err)
		}
	}

	return nil
}

// restoreUkiExtraFile copies a file saved by saveUkiExtraFiles into a UKI's addon directory.
func restoreUkiExtraFile(kernel string, fileName string, buildDir string, systemBootPartitionTmpDir string) error {
	filePath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, kernel+".efi.extra.d", fileName)
	err := file.Copy(ukiExtraFileSavePath(buildDir, kernel, fileName), filePath)
	if err != nil {
		return fmt.Errorf("failed to restore UKI addon directory file (%s):\n%w", filePath, err)
	}

	logger.Log.Infof("Kept UKI addon directory file: (%s)", filePath)
	return nil
}

// ukiExtraFileSavePath returns where a kernel's copy of a file of its UKI's addon directory is saved.
func ukiExtraFileSavePath(buildDir string, kernel string, fileName string) string {
	return filepath.Join(buildDir, UkiBuildDir, ukiExtraFilesDir, kernel, fileName)
}

// checkUkiLayoutsUnchanged returns an error if the command line or the addon directory of a base UKI changed since its
// layout was recorded.
func checkUkiLayoutsUnchanged(espDir string, kernelInfo map[string]UkiKernelInfo, buildDir string) error {
	for _, kernel := range slices.Sorted(maps.Keys(kernelInfo)) {
		info := kernelInfo[kernel]
		ukiFile := filepath.Join(espDir, UkiOutputDir, kernel+".efi")
		exists, err := file.PathExists(ukiFile)
		if err != nil {
			return fmt.Errorf("failed to check for UKI file (%s):\n%w", ukiFile, err)
		}

		// A kernel installed during customization has no UKI of its own yet.
		if info.BaseLayout == nil || !exists {
			continue
		}

		layout, err := readUkiLayout(ukiFile, buildDir)
		if err != nil {
			return fmt.Errorf("failed to read the layout of UKI (%s):\n%w", ukiFile, err)
		}

		changes := ukiLayoutChanges(info.BaseLayout, &layout)
		if len(changes) > 0 {
			return fmt.Errorf("%w (uki='%s', changes='%s')", ErrUKIAddonDirChanged, ukiFile,
				strings.Join(changes, ", "))
		}
	}

	return nil
}

// ukiLayoutsEqual reports whether two layouts are the same; a nil map equals an empty one.
func ukiLayoutsEqual(a *UkiLayout, b *UkiLayout) bool {
	if a == nil || b == nil {
		return a == b
	}

	return len(ukiLayoutChanges(a, b)) == 0
}

// ukiLayoutChanges lists what differs between a UKI layout and its base.
func ukiLayoutChanges(base *UkiLayout, layout *UkiLayout) []string {
	changes := []string(nil)
	if layout.MainCmdline != base.MainCmdline {
		changes = append(changes, "main UKI command line")
	}

	changes = append(changes, ukiFileChanges(base.Addons, layout.Addons)...)
	changes = append(changes, ukiFileChanges(base.ExtraFiles, layout.ExtraFiles)...)
	return changes
}

// ukiFileChanges lists the files added, removed or changed between two maps of file name to content.
func ukiFileChanges(base map[string]string, files map[string]string) []string {
	changes := []string(nil)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		baseContent, found := base[name]
		switch {
		case !found:
			changes = append(changes, "added "+name)
		case baseContent != files[name]:
			changes = append(changes, "changed "+name)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(base)) {
		if _, found := files[name]; !found {
			changes = append(changes, "removed "+name)
		}
	}

	return changes
}

// getFallbackUkiLayout returns the layout that a kernel with no UKI of its own inherits: the one the existing UKIs
// share, with <kernel>.addon.efi renamed. Like getFallbackKernelArgs, it fails if they disagree. Nil if none. It also
// returns the kernel whose UKI the layout is taken from.
func getFallbackUkiLayout(existingUkiKernelInfo map[string]UkiKernelInfo, kernel string) (*UkiLayout, string, error) {
	var fallback *UkiLayout
	fallbackKernel := ""
	for _, existingKernel := range slices.Sorted(maps.Keys(existingUkiKernelInfo)) {
		var layout *UkiLayout
		if baseLayout := existingUkiKernelInfo[existingKernel].BaseLayout; baseLayout != nil {
			renamed := renameUkiLayoutAddon(*baseLayout, ukiAddonFileName(existingKernel), ukiAddonFileName(kernel))
			layout = &renamed
		}

		if fallbackKernel == "" {
			fallback = layout
			fallbackKernel = existingKernel
			continue
		}

		if !ukiLayoutsEqual(fallback, layout) {
			return nil, "", fmt.Errorf("cannot pick a fallback UKI layout: kernels have divergent UKI layouts")
		}
	}

	return fallback, fallbackKernel, nil
}

// renameUkiLayoutAddon returns a copy of the layout with the addon oldName renamed to newName.
func renameUkiLayoutAddon(layout UkiLayout, oldName string, newName string) UkiLayout {
	cmdline, ok := layout.Addons[oldName]
	if !ok || oldName == newName {
		return layout
	}

	addons := maps.Clone(layout.Addons)
	delete(addons, oldName)
	addons[newName] = cmdline
	layout.Addons = addons
	return layout
}

// getUkiLayout splits a UKI's command line the way the base image's UKI did: an argument stays in the file it came
// from, a changed value takes the old one's place, and a new argument goes into the main UKI, or after the last
// same-named addon argument so that it still overrides it. An addon left with no arguments is dropped. Without a base
// layout, the whole command line goes into the main UKI.
func getUkiLayout(cmdline string, baseLayout *UkiLayout) (UkiLayout, error) {
	args, err := parseKernelCmdlineArgs(cmdline)
	if err != nil {
		return UkiLayout{}, fmt.Errorf("%w:\n%w", ErrUKISplitCmdline, err)
	}

	base := UkiLayout{}
	if baseLayout != nil {
		base = *baseLayout
	}

	// The main UKI first, then the addons by file name: the order in which systemd-stub concatenates them.
	fileNames := append([]string{""}, slices.Sorted(maps.Keys(base.Addons))...)
	baseArgs := make([][]ukiBaseArg, len(fileNames))
	for i, fileName := range fileNames {
		fileCmdline := base.MainCmdline
		if fileName != "" {
			fileCmdline = base.Addons[fileName]
			if fileCmdline == "" {
				// An addon without a command line carries something else (e.g. a devicetree).
				return UkiLayout{}, fmt.Errorf("%w:\nbase image addon (%s) has no kernel command line",
					ErrUKISplitCmdline, fileName)
			}
		}

		fileArgs, err := parseKernelCmdlineArgs(fileCmdline)
		if err != nil {
			return UkiLayout{}, fmt.Errorf("%w (file='%s'):\n%w", ErrUKISplitCmdline, fileName, err)
		}

		for _, arg := range fileArgs {
			baseArgs[i] = append(baseArgs[i], ukiBaseArg{arg: arg})
		}
	}

	// First, each argument identical to a base argument takes that argument's place.
	placed := make([]bool, len(args))
	for i, arg := range args {
		placed[i] = placeUkiArg(baseArgs, arg, func(baseArg grubConfigLinuxArg) bool {
			return baseArg.Arg == arg.Arg
		})
	}

	// Then each other argument takes the place of an unused base argument of the same name, or is appended to the
	// last file that has that name (the main UKI if none does), so that it still overrides it.
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
		// Unused base arguments of that name in more than one file: no way to tell which one this replaces.
		if len(fileIndexes) > 1 {
			return UkiLayout{}, fmt.Errorf("%w (arg='%s')", ErrUKIAmbiguousArg, arg.Arg)
		}

		if !placeUkiArg(baseArgs, arg, func(baseArg grubConfigLinuxArg) bool { return baseArg.Name == arg.Name }) {
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
			logger.Log.Infof("UKI addon (%s) has no kernel command-line args left; not adding it", fileName)

		default:
			layout.Addons[fileName] = GrubArgsToString(fileArgs)
		}
	}

	return layout, nil
}

// placeUkiArg gives arg the place of the first unused base arg that matches, in command-line order, and reports
// whether it found one.
func placeUkiArg(baseArgs [][]ukiBaseArg, arg grubConfigLinuxArg, matches func(grubConfigLinuxArg) bool) bool {
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

// parseKernelCmdlineArgs splits a kernel command line into its arguments.
func parseKernelCmdlineArgs(cmdline string) ([]grubConfigLinuxArg, error) {
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
			// The parsed arg is truncated at a variable expansion, so it can't be rewritten.
			return nil, fmt.Errorf("kernel command-line arg (%s) contains a variable expansion", arg.Arg)
		}
	}

	return args, nil
}
