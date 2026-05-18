// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package bls parses Boot Loader Specification (BLS) Type #1 entry files
// (/loader/entries/*.conf), which contain simple line-based key/value pairs
// (e.g. `title`, `linux`, `initrd`, `options`).
//
// See https://uapi-group.org/specifications/specs/boot_loader_specification/
//
// The parser intentionally only models the file-level structure. Interpretation
// of individual values (e.g. parsing an `options` value as a kernel command
// line) is left to callers, because those concerns are not part of the BLS
// spec itself.
package bls

import "strings"

// Field is one key-value pair from a BLS entry file.
type Field struct {
	Key   string
	Value string
}

// Line is one logical line of a BLS entry file, with byte offsets into the source content so callers can splice
// in-place rewrites while preserving the rest of the file verbatim.
type Line struct {
	Field
	ContentStart int
	ContentEnd   int
}

// ParseFields parses a BLS entry file into its key-value pairs, in source order, with blank lines and comments
// dropped. This is the canonical entry point for code that interprets BLS values.
func ParseFields(content string) []Field {
	lines := ParseLines(content)
	fields := make([]Field, 0, len(lines))
	for _, line := range lines {
		if line.Key == "" {
			continue
		}
		fields = append(fields, line.Field)
	}
	return fields
}

// ParseLines parses a BLS entry file into logical lines, following the rules used by systemd's `boot_entry_load_type1`
// in src/shared/bootspec.c:
//
//   - Lines are LF-terminated; CRLF is tolerated.
//   - Each line is stripped of leading and trailing whitespace (space, tab) before further processing.
//   - Empty lines and lines whose first non-whitespace character is '#' are returned with Key="".
//   - Otherwise the line is split on the FIRST run of whitespace into a key and a value. The value is taken verbatim to
//     end-of-line. '$', '"', "'", and '\\' inside a value are LITERAL characters, not subject to grub-style variable
//     expansion, quote stripping, or escape processing. (This is the key difference from grub.cfg syntax.)
//
// Prefer ParseFields unless you need byte offsets for in-place rewriting.
func ParseLines(content string) []Line {
	if content == "" {
		return nil
	}
	var lines []Line
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

		line := Line{ContentStart: start, ContentEnd: end}
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

// isBLSRescueEntryTitle reports whether the given BLS entry title looks like a rescue entry emitted by systemd's
// `kernel-install` (via 90-loaderentry.install). Those entries hardcode the substring "0-rescue-<machine-id>" inside
// the title.
func IsRescueEntryTitle(title string) bool {
	return strings.Contains(strings.ToLower(title), "rescue")
}
