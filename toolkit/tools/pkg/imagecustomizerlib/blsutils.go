// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Helpers for working with Boot Loader Specification (BLS) entries that depend
// on higher-level concepts in this package (grub cmdline parsing, verity
// metadata, etc.). The file-format parser itself lives in internal/bls.
//
// See https://uapi-group.org/specifications/specs/boot_loader_specification/

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/bls"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// parseBLSOptionsValue parses the string value of a BLS `options` key as a kernel command line. Per the kernel's
// cmdline parsing rules (lib/cmdline.c:next_arg), tokens are whitespace-separated with double-quote grouping. Even
// special characters (e.g. `;`, `|`, `&`, `<`, `>`, `{`, `}`) are passed through verbatim. We return
// []grubConfigLinuxArg but deliberately do not use the grub tokenizer to align with the kernel's parsing behavior.
func parseBLSOptionsValue(value string) []grubConfigLinuxArg {
	var args []grubConfigLinuxArg
	var cur strings.Builder
	inQuotes := false

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		tok := cur.String()
		cur.Reset()
		name := tok
		val := ""
		if eq := strings.IndexByte(tok, '='); eq >= 0 {
			name = tok[:eq]
			val = tok[eq+1:]
		}
		args = append(args, grubConfigLinuxArg{Arg: tok, Name: name, Value: val})
	}

	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case !inQuotes && (c == ' ' || c == '\t'):
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return args
}

// readKernelCmdlinesFromBLSEntries reads Boot Loader Specification (BLS) entries in {bootDir}/loader/entries/*.conf,
// extracting a kernel-to-cmdline mapping for non-recovery entries.
func readKernelCmdlinesFromBLSEntries(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	entriesDir := filepath.Join(bootDir, "loader", "entries")
	entries, err := os.ReadDir(entriesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read BLS entries directory (%s):\n%w", entriesDir, err)
	}

	kernelToArgs := make(map[string][]grubConfigLinuxArg)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			logger.Log.Debugf("Skipping non-.conf BLS entry file (%s) in directory (%s)", entry.Name(), entriesDir)
			continue
		}

		absPath := filepath.Join(entriesDir, entry.Name())
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read BLS entry file (%s):\n%w", absPath, err)
		}

		var linux string
		var title string
		var args []grubConfigLinuxArg

		for _, field := range bls.ParseFields(string(content)) {
			switch field.Key {
			case "linux":
				if linux != "" {
					return nil, fmt.Errorf("duplicate key (%s) in BLS entry (%s)", field.Key, absPath)
				}
				if field.Value == "" {
					return nil, fmt.Errorf("BLS entry (%s) 'linux' key has empty value", absPath)
				}
				linux = filepath.Base(field.Value)
			case "title":
				title = field.Value
			case "efi", "uki", "uki-url":
				return nil, fmt.Errorf("BLS entry (%s) uses '%s' key, which is not supported", absPath, field.Key)
			case "options":
				// "options" may appear multiple times per BLS spec.
				args = append(args, parseBLSOptionsValue(field.Value)...)
			}
		}

		if linux == "" {
			return nil, fmt.Errorf("BLS entry (%s) is missing 'linux' key", absPath)
		}

		// Entries without titles are treated as normal entries.
		if bls.IsRescueEntryTitle(title) {
			logger.Log.Debugf("Skipping recovery/rescue BLS entry with title (%s) in file (%s)", title, absPath)
			continue
		}

		if _, exists := kernelToArgs[linux]; exists {
			return nil, fmt.Errorf("duplicate BLS entries for kernel (%s) in (%s)", linux, entriesDir)
		}

		kernelToArgs[linux] = args
	}

	return kernelToArgs, nil
}

// readNonRecoveryKernelCmdlinesFromBLS reads the first non-recovery kernel's command-line
// arguments from BLS entry files.
func readNonRecoveryKernelCmdlinesFromBLS(bootDir string, argNames []string) (map[string]string, error) {
	kernelToArgs, err := readKernelCmdlinesFromBLSEntries(bootDir)
	if err != nil {
		return nil, err
	}

	if len(kernelToArgs) > 1 {
		return nil, fmt.Errorf("expected 1 non-recovery BLS entry, found %d", len(kernelToArgs))
	}

	for _, args := range kernelToArgs {
		return filterKernelArgsByName(args, argNames), nil
	}

	return nil, fmt.Errorf("no non-recovery BLS entries found")
}

// updateBLSEntriesForVerity updates BLS entry options with verity kernel args.
func updateBLSEntriesForVerity(verityMetadata []verityDeviceMetadata, bootDir string,
	partitions []diskutils.PartitionInfo, buildDir string, bootUuid string,
) error {
	newArgs, err := constructVerityKernelCmdlineArgs(verityMetadata, partitions, bootUuid)
	if err != nil {
		return fmt.Errorf("failed to generate verity kernel arguments:\n%w", err)
	}

	argsToRemove := slices.Clone(verityKernelArgsToRemove)

	rootExists := slices.ContainsFunc(verityMetadata, func(metadata verityDeviceMetadata) bool {
		return metadata.name == imagecustomizerapi.VerityRootDeviceName
	})
	if rootExists {
		argsToRemove = append(argsToRemove, "root")
		newArgs = append(newArgs, "root="+imagecustomizerapi.VerityRootDevicePath)
	}

	entriesDir := filepath.Join(bootDir, "loader", "entries")
	entries, err := os.ReadDir(entriesDir)
	if err != nil {
		return fmt.Errorf("failed to read BLS entries directory (%s):\n%w", entriesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		absPath := filepath.Join(entriesDir, entry.Name())
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read BLS entry file (%s):\n%w", absPath, err)
		}

		updatedContent, err := updateBLSEntryOptions(string(content), argsToRemove, newArgs)
		if err != nil {
			return fmt.Errorf("failed to update BLS entry options (%s):\n%w", absPath, err)
		}

		err = os.WriteFile(absPath, []byte(updatedContent), 0o644)
		if err != nil {
			return fmt.Errorf("failed to write BLS entry file (%s):\n%w", absPath, err)
		}
	}

	return nil
}

// updateBLSEntryOptions updates the options line in a BLS entry, removing old args and adding new ones.
//
// Implementation notes:
//   - The file-level structure is parsed per the BLS spec (line-based, value taken verbatim to end-of-line). The
//     *value* of an `options` line is then parsed as a kernel command line with the grub tokenizer, so quoted values
//     like rd.cmdline="foo bar" survive the remove/append round-trip instead of being shredded on whitespace.
//   - Per BLS spec an entry may contain multiple "options" lines whose values are concatenated. argsToRemove is applied
//     to every options line. newArgs is appended only to the last one.
//   - Each existing options line is replaced byte-for-byte at the source location of its trimmed content, so leading
//     whitespace, comments, and unrelated lines are preserved exactly.
//   - If no options line exists, a new one is appended.
func updateBLSEntryOptions(content string, argsToRemove []string, newArgs []string) (string, error) {
	lines := bls.ParseLines(content)

	// Collect indices of all "options" lines.
	var optionsIdx []int
	for i, line := range lines {
		if line.Key == "options" {
			optionsIdx = append(optionsIdx, i)
		}
	}

	if len(optionsIdx) == 0 {
		newLine := "options"
		if len(newArgs) > 0 {
			newLine += " " + GrubArgsToString(newArgs)
		}
		if content != "" && !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		return content + newLine + "\n", nil
	}

	result := content

	// Splice in reverse byte order so earlier offsets remain valid as we make replacements.
	for i := len(optionsIdx) - 1; i >= 0; i-- {
		line := lines[optionsIdx[i]]
		args := parseBLSOptionsValue(line.Value)
		argStrings := make([]string, 0, len(args))
		for _, arg := range args {
			if slices.Contains(argsToRemove, arg.Name) {
				continue
			}
			argStrings = append(argStrings, arg.Arg)
		}

		// Append new args only to the last options line.
		if i == len(optionsIdx)-1 {
			argStrings = append(argStrings, newArgs...)
		}

		replacement := "options"
		if len(argStrings) > 0 {
			replacement += " " + GrubArgsToString(argStrings)
		}

		result = result[:line.ContentStart] + replacement + result[line.ContentEnd:]
	}

	return result, nil
}
