// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateBLSEntryOptions(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		argsToRemove []string
		newArgs      []string
		want         string
	}{
		{
			name: "preserves quoted value with embedded space",
			content: "title Foo\n" +
				"linux /vmlinuz\n" +
				"options root=/dev/sda1 rd.cmdline=\"foo bar\" quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      []string{"verity=1"},
			// rd.cmdline="foo bar" must remain a single arg; only "quiet"
			// should disappear. GrubArgsToString re-quotes as double-quoted.
			want: "title Foo\n" +
				"linux /vmlinuz\n" +
				"options root=/dev/sda1 \"rd.cmdline=foo bar\" verity=1\n",
		},
		{
			name: "removes named arg whose value is quoted with spaces",
			content: "title Foo\n" +
				"options root=/dev/sda1 rd.cmdline=\"foo bar\" quiet\n",
			argsToRemove: []string{"rd.cmdline"},
			newArgs:      nil,
			// The whole rd.cmdline=... token must be removed atomically.
			// Pre-fix strings.Fields would have dropped only `rd.cmdline="foo`
			// and left a dangling `bar"` token.
			want: "title Foo\n" +
				"options root=/dev/sda1 quiet\n",
		},
		{
			name: "tab between key and value is recognized",
			content: "title Foo\n" +
				"options\troot=/dev/sda1 quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options root=/dev/sda1 verity=1\n",
		},
		{
			name: "mixed tab-then-space is recognized",
			content: "title Foo\n" +
				"options\troot=/dev/sda1 quiet\n",
			argsToRemove: []string{"root"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options quiet verity=1\n",
		},
		{
			name: "multiple options lines: remove from all, append to last",
			content: "title Foo\n" +
				"options root=/dev/sda1 stale=yes\n" +
				"options quiet stale=also\n",
			argsToRemove: []string{"stale"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options root=/dev/sda1\n" +
				"options quiet verity=1\n",
		},
		{
			name:         "no options line: appended preserving trailing newline",
			content:      "title Foo\nlinux /vmlinuz\n",
			argsToRemove: []string{"stale"},
			newArgs:      []string{"verity=1"},
			want:         "title Foo\nlinux /vmlinuz\noptions verity=1\n",
		},
		{
			name:         "no options line and no trailing newline: newline inserted",
			content:      "title Foo\nlinux /vmlinuz",
			argsToRemove: nil,
			newArgs:      []string{"verity=1"},
			want:         "title Foo\nlinux /vmlinuz\noptions verity=1\n",
		},
		{
			name: "comments and unrelated keys are preserved",
			content: "# a comment\n" +
				"title Foo\n" +
				"linux /vmlinuz\n" +
				"options root=/dev/sda1\n" +
				"initrd /initrd.img\n",
			argsToRemove: nil,
			newArgs:      []string{"verity=1"},
			want: "# a comment\n" +
				"title Foo\n" +
				"linux /vmlinuz\n" +
				"options root=/dev/sda1 verity=1\n" +
				"initrd /initrd.img\n",
		},
		{
			// argsToRemove that matches nothing must be a true no-op semantically.
			// The args list survives the tokenize -> filter -> re-emit round-trip.
			name: "argsToRemove with no matches preserves args",
			content: "title Foo\n" +
				"options root=/dev/sda1 quiet\n",
			argsToRemove: []string{"nonexistent", "alsogone"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options root=/dev/sda1 quiet\n",
		},
		{
			// nil argsToRemove + nil newArgs must still survive the round-trip
			// without losing or duplicating args.
			name: "nil args lists: passthrough preserves all args",
			content: "title Foo\n" +
				"options root=/dev/sda1 ro quiet\n",
			argsToRemove: nil,
			newArgs:      nil,
			want: "title Foo\n" +
				"options root=/dev/sda1 ro quiet\n",
		},
		{
			// When every existing arg is removed and no new args are added the
			// options line must collapse to a bare "options" rather than being
			// dropped or producing trailing whitespace.
			name: "all args removed and no newArgs yields bare options line",
			content: "title Foo\n" +
				"options root=/dev/sda1 quiet\n",
			argsToRemove: []string{"root", "quiet"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options\n",
		},
		{
			// Grub permits single-quoted values; the tokenizer normalizes them
			// during read. On re-emit GrubArgsToString always uses double quotes,
			// so a single-quoted input becomes double-quoted output but the
			// arg's name/value identity is preserved.
			name: "single-quoted value is preserved as one arg",
			content: "title Foo\n" +
				"options root=/dev/sda1 rd.cmdline='foo bar' quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options root=/dev/sda1 \"rd.cmdline=foo bar\"\n",
		},
		{
			// A quoted value containing '=' characters must remain a single arg.
			// ParseCommandLineArgs splits the assembled token on the FIRST '='
			// only, so name="rd.cmdline" and value="a=b c=d".
			name: "quoted value containing '=' preserved as one arg",
			content: "title Foo\n" +
				"options \"rd.cmdline=a=b c=d\" quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options \"rd.cmdline=a=b c=d\"\n",
		},
		{
			// Any leading indentation before the "options" keyword sits outside
			// the splice range and must be preserved verbatim.
			name: "indented options line: leading whitespace preserved",
			content: "title Foo\n" +
				"  options root=/dev/sda1\n",
			argsToRemove: nil,
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"  options root=/dev/sda1 verity=1\n",
		},
		{
			// Empty input + newArgs must produce a single options line with
			// trailing newline (the no-options-line append path).
			name:         "empty content with newArgs yields just options line",
			content:      "",
			argsToRemove: nil,
			newArgs:      []string{"verity=1"},
			want:         "options verity=1\n",
		},
		{
			// Empty input + nil newArgs must produce a bare options line.
			name:         "empty content with no newArgs yields bare options line",
			content:      "",
			argsToRemove: nil,
			newArgs:      nil,
			want:         "options\n",
		},
		{
			// With multiple options lines, the writer applies argsToRemove to
			// every line but appends newArgs only to the last. When the first
			// line becomes empty it must collapse to a bare "options" rather
			// than being dropped (preserving BLS-spec multi-line semantics).
			name: "multiple options lines: emptied line collapses, newArgs on last",
			content: "title Foo\n" +
				"options root=/dev/sda1\n" +
				"options quiet\n",
			argsToRemove: []string{"root"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options\n" +
				"options quiet verity=1\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updateBLSEntryOptions(tc.content, tc.argsToRemove, tc.newArgs)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReadKernelCmdlinesFromBLSEntries(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		wantKernels   []string
		wantArgsFor   map[string][]string // kernel -> expected arg strings (Arg field) in order
		wantErrSubstr string
	}{
		{
			name: "preserves quoted value with embedded space in options",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1 rd.cmdline=\"foo bar\" quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "rd.cmdline=foo bar", "quiet"},
			},
		},
		{
			name: "tab between key and value is recognized",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux\t/vmlinuz-6.6\n" +
					"options\troot=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "mixed tab-then-space separator is recognized",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options\troot=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "multiple options lines are concatenated per BLS spec",
			files: map[string]string{
				"azl.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n" +
					"options quiet rhgb\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet", "rhgb"},
			},
		},
		{
			name: "recovery entries are skipped, not errored",
			files: map[string]string{
				"normal.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
				"rescue.conf": "title Azure Linux 0-rescue-abc123\n" +
					"linux /vmlinuz-6.6-rescue\n" +
					"options root=/dev/sda1 systemd.unit=rescue.target\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1"},
			},
		},
		{
			name: "comments and blank lines are tolerated",
			files: map[string]string{
				"azl.conf": "# An Azure Linux BLS entry\n" +
					"\n" +
					"title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"# kernel command line:\n" +
					"options root=/dev/sda1 quiet\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
			},
		},
		{
			name: "'efi' key produces an error",
			files: map[string]string{
				"efi.conf": "title Azure Linux\n" +
					"efi /EFI/Linux/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'efi' key",
		},
		{
			name: "'uki' key produces an error",
			files: map[string]string{
				"uki.conf": "title Azure Linux\n" +
					"uki /EFI/Linux/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'uki' key",
		},
		{
			name: "'uki-url' key produces an error",
			files: map[string]string{
				"uki-url.conf": "title Azure Linux\n" +
					"uki-url https://example.com/vmlinuz.efi\n",
			},
			wantErrSubstr: "uses 'uki-url' key",
		},
		{
			name: "no 'linux' key produces an error",
			files: map[string]string{
				"bad.conf": "title Azure Linux\n" +
					"options root=/dev/sda1\n",
			},
			wantErrSubstr: "missing 'linux' key",
		},
		{
			name: "no 'title' key is treated as a normal entry",
			files: map[string]string{
				"bad.conf": "linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bootDir := t.TempDir()
			entriesDir := filepath.Join(bootDir, "loader", "entries")
			err := os.MkdirAll(entriesDir, 0o755)
			if !assert.NoError(t, err) {
				return
			}
			for name, content := range tc.files {
				err := os.WriteFile(filepath.Join(entriesDir, name), []byte(content), 0o644)
				if !assert.NoError(t, err) {
					return
				}
			}

			got, err := readKernelCmdlinesFromBLSEntries(bootDir)
			if tc.wantErrSubstr != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tc.wantErrSubstr)
				}
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			gotKernels := make([]string, 0, len(got))
			for k := range got {
				gotKernels = append(gotKernels, k)
			}
			assert.ElementsMatch(t, tc.wantKernels, gotKernels)

			for kernel, wantArgs := range tc.wantArgsFor {
				args, ok := got[kernel]
				if !assert.True(t, ok, "expected kernel %q in result", kernel) {
					continue
				}
				gotArgStrings := make([]string, 0, len(args))
				for _, arg := range args {
					gotArgStrings = append(gotArgStrings, arg.Arg)
				}
				assert.Equal(t, wantArgs, gotArgStrings, "args for kernel %q", kernel)
			}
		})
	}
}

func TestParseBLSOptionsValue(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		wantArg       []string // expected .Arg fields, in order; ignored if wantErrSubstr is set
		wantErrSubstr string   // if non-empty, parse must fail with this substring in the error
	}{
		{
			name:    "empty value yields no args",
			value:   "",
			wantArg: []string{},
		},
		{
			name:    "plain args",
			value:   "console=ttyS0 root=/dev/sda1",
			wantArg: []string{"console=ttyS0", "root=/dev/sda1"},
		},
		{
			name:    "quoted value with embedded space is one arg",
			value:   `rd.cmdline="foo bar" quiet`,
			wantArg: []string{"rd.cmdline=foo bar", "quiet"},
		},
		{
			// Quoted grub metacharacters are literal inside the WORD subword,
			// so the whole value parses as one arg.
			name:    "quoted value with embedded semicolon is one arg",
			value:   `rd.cmdline="foo;bar" quiet`,
			wantArg: []string{"rd.cmdline=foo;bar", "quiet"},
		},
		{
			// An unquoted grub metacharacter in a BLS options value cannot be
			// faithfully represented as a kernel cmdline arg.
			name:          "unquoted semicolon is rejected",
			value:         "console=ttyS0;quiet",
			wantErrSubstr: "unexpected grub metacharacter",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args, err := parseBLSOptionsValue(tc.value)
			if tc.wantErrSubstr != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tc.wantErrSubstr)
				}
				return
			}
			assert.NoError(t, err)
			gotArgs := make([]string, 0, len(args))
			for _, a := range args {
				gotArgs = append(gotArgs, a.Arg)
			}
			assert.Equal(t, tc.wantArg, gotArgs)
		})
	}
}
