// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Helpers for working with Boot Loader Specification (BLS) entries.
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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

// FindBlsCfg reports whether the given grub.cfg lines contain a `blscfg`
// command, indicating that the distro uses BLS entries for its boot menu.
func FindBlsCfg(grubLines []grub.Line) bool {
	for _, line := range grubLines {
		if len(line.Tokens) >= 1 && grub.IsTokenKeyword(line.Tokens[0], "blscfg") {
			return true
		}
	}
	return false
}

// parseBLSLinuxValue parses the value following a "linux" key in a BLS entry.
func parseBLSLinuxValue(lineTokens []grub.Token) (string, error) {
	if len(lineTokens) != 2 {
		return "", fmt.Errorf("expected 1 value token, found %d", len(lineTokens)-1)
	}
	valueToken := lineTokens[1]
	if valueToken.Type != grub.WORD {
		return "", fmt.Errorf("unexpected non-word value token: %v", valueToken.RawContent)
	}
	if len(valueToken.SubWords) != 1 {
		return "", fmt.Errorf("expected 1 subword, found %d", len(valueToken.SubWords))
	}
	sw := valueToken.SubWords[0]
	if sw.Type != grub.KEYWORD_STRING && sw.Type != grub.STRING {
		return "", fmt.Errorf("unexpected non-string subword: %v", sw.RawContent)
	}
	return sw.Value, nil
}

// parseBLSTitleValue parses the value following a "title" key in a BLS entry.
func parseBLSTitleValue(lineTokens []grub.Token) (string, error) {
	if len(lineTokens) < 2 {
		return "", fmt.Errorf("expected at least 1 value token, found 0")
	}
	valueParts := make([]string, 0, len(lineTokens)-1)
	for _, valueToken := range lineTokens[1:] {
		if valueToken.Type != grub.WORD {
			return "", fmt.Errorf("unexpected non-word value token: %v", valueToken.RawContent)
		}
		if len(valueToken.SubWords) < 1 {
			return "", fmt.Errorf("expected at least 1 subword, found 0")
		}
		var sb strings.Builder
		for _, sw := range valueToken.SubWords {
			if sw.Type != grub.KEYWORD_STRING && sw.Type != grub.STRING {
				return "", fmt.Errorf("unexpected non-string subword: %v", sw.RawContent)
			}
			sb.WriteString(sw.Value)
		}
		valueParts = append(valueParts, sb.String())
	}
	return strings.Join(valueParts, " "), nil
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

		// BLS entries are grub-syntax key/value pairs, so we can leverage the grub parsing code here.
		tokens, err := grub.TokenizeConfig(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to tokenize BLS entry (%s):\n%w", absPath, err)
		}
		lines := grub.SplitTokensIntoLines(tokens)

		var linux string
		var title string
		var args []grubConfigLinuxArg

		for _, line := range lines {
			if len(line.Tokens) == 0 {
				continue
			}

			keyToken := line.Tokens[0]
			if keyToken.Type != grub.WORD {
				return nil, fmt.Errorf("unexpected non-word token in BLS entry (%s): %v", absPath, keyToken.RawContent)
			}
			if len(keyToken.SubWords) != 1 {
				return nil, fmt.Errorf("unexpected token with multiple subwords in BLS entry (%s): %v", absPath, keyToken.RawContent)
			}
			if keyToken.SubWords[0].Type != grub.KEYWORD_STRING {
				return nil, fmt.Errorf("unexpected non-string token in BLS entry (%s): %v", absPath, keyToken.SubWords[0].RawContent)
			}
			key := keyToken.SubWords[0].Value

			switch key {
			case "linux":
				if linux != "" {
					return nil, fmt.Errorf("duplicate key (%s) in BLS entry (%s)", key, absPath)
				}
				linuxValue, err := parseBLSLinuxValue(line.Tokens)
				if err != nil {
					return nil, fmt.Errorf("failed to parse BLS key (%s) for entry (%s):\n%w", key, absPath, err)
				}
				linux = filepath.Base(linuxValue)
			case "title":
				titleValue, err := parseBLSTitleValue(line.Tokens)
				if err != nil {
					return nil, fmt.Errorf("failed to parse BLS key (%s) for entry (%s):\n%w", key, absPath, err)
				}
				title = titleValue
			case "efi", "uki", "uki-url":
				return nil, fmt.Errorf("BLS entry (%s) uses '%s' key, which is not supported", absPath, key)
			case "options":
				// "options" may appear multiple times per BLS spec.
				lineArgs, err := ParseCommandLineArgs(line.Tokens[1:])
				if err != nil {
					return nil, fmt.Errorf("failed to parse BLS key (%s) for entry (%s):\n%w", key, absPath, err)
				}
				args = append(args, lineArgs...)
			}
		}

		if linux == "" {
			return nil, fmt.Errorf("BLS entry (%s) is missing 'linux' key", absPath)
		}

		// Entries without titles are treated as normal entries.
		if isRecoveryOrRescueTitle(title) {
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
//   - Uses the grub tokenizer so the reader (readKernelCmdlinesFromBLSEntries) and this
//     writer share the same quoting/whitespace rules. This preserves quoted values like
//     rd.cmdline="foo bar" across a remove/append round-trip instead of shredding them
//     on whitespace.
//   - Per BLS spec an entry may contain multiple "options" lines whose values are
//     concatenated. argsToRemove is applied to every options line; newArgs is appended
//     only to the last one (so re-runs converge instead of duplicating).
//   - Each existing options line is replaced byte-for-byte at the source location of its
//     tokens, so surrounding lines, comments, indentation and the trailing newline are
//     preserved exactly.
//   - If no options line exists, a new one is appended, matching the file's existing
//     trailing-newline convention.
func updateBLSEntryOptions(content string, argsToRemove []string, newArgs []string) (string, error) {
	tokens, err := grub.TokenizeConfig(content)
	if err != nil {
		return "", fmt.Errorf("failed to tokenize BLS entry:\n%w", err)
	}

	lines := grub.SplitTokensIntoLines(tokens)
	optionsLines := grub.FindCommandAll(lines, "options")

	if len(optionsLines) == 0 {
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

	// Splice in reverse order so earlier byte offsets in result remain valid as we make replacements.
	for i := len(optionsLines) - 1; i >= 0; i-- {
		line := optionsLines[i]

		// line.Tokens[0] is the "options" keyword; the rest are the args.
		args, err := ParseCommandLineArgs(line.Tokens[1:])
		if err != nil {
			return "", fmt.Errorf("failed to parse BLS options line:\n%w", err)
		}

		argStrings := make([]string, 0, len(args))
		for _, arg := range args {
			if slices.Contains(argsToRemove, arg.Name) {
				continue
			}
			argStrings = append(argStrings, arg.Arg)
		}

		// Append new args only to the last options line.
		if i == len(optionsLines)-1 {
			argStrings = append(argStrings, newArgs...)
		}

		replacement := "options"
		if len(argStrings) > 0 {
			replacement += " " + GrubArgsToString(argStrings)
		}

		start := line.Tokens[0].Loc.Start.Index
		end := line.Tokens[len(line.Tokens)-1].Loc.End.Index
		result = result[:start] + replacement + result[end:]
	}

	return result, nil
}
