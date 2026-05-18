// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Helpers for working with Boot Loader Specification (BLS) entries.
// See https://uapi-group.org/specifications/specs/boot_loader_specification/
//
// The parsers in this file handle the text entry files under
// /loader/entries/*.conf, which contain simple line-based key/value pairs
// (e.g. `title`, `linux`, `initrd`, `options`).

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

// blsField is one key-value pair from a BLS entry file.
type blsField struct {
	Key   string
	Value string
}

// blsLine is one logical line of a BLS entry file, with byte offsets into the source content so callers can splice
// in-place rewrites while preserving the rest of the file verbatim.
type blsLine struct {
	blsField
	ContentStart int
	ContentEnd   int
}

// FindBlsCfg reports whether the given grub.cfg lines contain a `blscfg` command, indicating that the distro uses BLS
// entries for its boot menu.
func FindBlsCfg(grubLines []grub.Line) bool {
	for _, line := range grubLines {
		if len(line.Tokens) >= 1 && grub.IsTokenKeyword(line.Tokens[0], "blscfg") {
			return true
		}
	}
	return false
}

// parseBLSFields parses a BLS entry file into its key-value pairs, in source order, with blank lines and comments
// dropped. This is the canonical entry point for code that interprets BLS values.
func parseBLSFields(content string) []blsField {
	lines := parseBLSLines(content)
	fields := make([]blsField, 0, len(lines))
	for _, line := range lines {
		if line.Key == "" {
			continue
		}
		fields = append(fields, line.blsField)
	}
	return fields
}

// parseBLSLines parses a BLS entry file into logical lines, following the rules used by systemd's
// `boot_entry_load_type1` in src/shared/bootspec.c:
//
//   - Lines are LF-terminated; CRLF is tolerated.
//   - Each line is stripped of leading and trailing whitespace (space, tab) before further processing.
//   - Empty lines and lines whose first non-whitespace character is '#' are returned with Key="".
//   - Otherwise the line is split on the FIRST run of whitespace into a key and a value. The value is taken verbatim to
//     end-of-line. '$', '"', "'", and '\\' inside a value are LITERAL characters, not subject to grub-style variable
//     expansion, quote stripping, or escape processing. (This is the key difference from grub.cfg syntax.)
//
// Prefer parseBLSFields unless you need byte offsets for in-place rewriting.
func parseBLSLines(content string) []blsLine {
	if content == "" {
		return nil
	}
	var lines []blsLine
	pos := 0
	for pos < len(content) {
		rel := strings.IndexByte(content[pos:], '\n')
		var lineEnd int
		if rel < 0 {
			lineEnd = len(content)
		} else {
			lineEnd = pos + rel
		}

		// Trim trailing whitespace and any CR (for CRLF tolerance).
		end := lineEnd
		for end > pos && (content[end-1] == ' ' || content[end-1] == '\t' || content[end-1] == '\r') {
			end--
		}

		// Trim leading whitespace.
		start := pos
		for start < end && (content[start] == ' ' || content[start] == '\t') {
			start++
		}

		line := blsLine{ContentStart: start, ContentEnd: end}
		if start < end && content[start] != '#' {
			// Split on the first whitespace run.
			sep := start
			for sep < end && content[sep] != ' ' && content[sep] != '\t' {
				sep++
			}
			line.Key = content[start:sep]

			// Trim leading whitespace of value.
			valStart := sep
			for valStart < end && (content[valStart] == ' ' || content[valStart] == '\t') {
				valStart++
			}
			line.Value = content[valStart:end]
		}
		lines = append(lines, line)

		if rel < 0 {
			break
		}
		pos = lineEnd + 1
	}
	return lines
}

// parseBLSOptionsValue parses the verbatim value of a BLS `options` key as a kernel command line. The kernel cmdline
// uses double-quote-grouped, whitespace-separated tokens, which the grub tokenizer handles correctly. But the grub
// tokenizer also treats unquoted grub metacharacters (e.g. `;`, `|`, `&`, `<`, `>`, `{`, `}`) as framing tokens, which
// the kernel does not. Such a token in a BLS options value would silently corrupt the resulting cmdline, so we reject
// it explicitly rather than guess at the author's intent.
func parseBLSOptionsValue(value string) ([]grubConfigLinuxArg, error) {
	tokens, err := grub.TokenizeConfig(value)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize kernel cmdline:\n%w", err)
	}
	for _, t := range tokens {
		if t.Type != grub.WORD {
			return nil, fmt.Errorf(
				"unexpected grub metacharacter (%s) in BLS options value (%q): missing quotes around value?",
				grub.TokenTypeString(t.Type), t.RawContent)
		}
	}
	return ParseCommandLineArgs(tokens)
}

// isBLSRescueEntryTitle reports whether the given BLS entry title looks like a rescue entry emitted by systemd's
// `kernel-install` (via 90-loaderentry.install). Those entries hardcode the substring "0-rescue-<machine-id>" inside
// the title.
func isBLSRescueEntryTitle(title string) bool {
	return strings.Contains(strings.ToLower(title), "rescue")
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

		for _, field := range parseBLSFields(string(content)) {
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
				lineArgs, err := parseBLSOptionsValue(field.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to parse BLS key (%s) for entry (%s):\n%w", field.Key, absPath, err)
				}
				args = append(args, lineArgs...)
			}
		}

		if linux == "" {
			return nil, fmt.Errorf("BLS entry (%s) is missing 'linux' key", absPath)
		}

		// Entries without titles are treated as normal entries.
		if isBLSRescueEntryTitle(title) {
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
	lines := parseBLSLines(content)

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

		args, err := parseBLSOptionsValue(line.Value)
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
