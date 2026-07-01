// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
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
			// The Linux kernel cmdline parser only recognizes double quotes for grouping.
			name: "single-quoted value is treated as literal characters per kernel cmdline semantics",
			content: "title Foo\n" +
				"options root=/dev/sda1 rd.cmdline='foo bar' quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options root=/dev/sda1 \"rd.cmdline='foo\" \"bar'\"\n",
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
		{
			name: "unquoted semicolon: edit applies and value is re-quoted on emit",
			content: "title Foo\n" +
				"options root=/dev/sda1 console=ttyS0;115200 quiet\n",
			argsToRemove: []string{"quiet"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options root=/dev/sda1 \"console=ttyS0;115200\" verity=1\n",
		},
		{
			name: "unquoted semicolon: arg whose value contains semicolon is removable atomically",
			content: "title Foo\n" +
				"options root=/dev/sda1 console=ttyS0;115200 quiet\n",
			argsToRemove: []string{"console"},
			newArgs:      nil,
			want: "title Foo\n" +
				"options root=/dev/sda1 quiet\n",
		},
		{
			name: "newArgs containing whitespace are quoted on append",
			content: "title Foo\n" +
				"options root=/dev/sda1\n",
			argsToRemove: nil,
			newArgs:      []string{"rd.cmdline=foo bar"},
			want: "title Foo\n" +
				"options root=/dev/sda1 \"rd.cmdline=foo bar\"\n",
		},
		{
			name:         "CRLF line endings are preserved across the rewrite",
			content:      "title Foo\r\noptions root=/dev/sda1 quiet\r\n",
			argsToRemove: []string{"quiet"},
			newArgs:      []string{"verity=1"},
			want:         "title Foo\r\noptions root=/dev/sda1 verity=1\r\n",
		},
		{
			name:         "options line at EOF without trailing newline keeps shape",
			content:      "title Foo\noptions root=/dev/sda1 quiet",
			argsToRemove: []string{"quiet"},
			newArgs:      []string{"verity=1"},
			want:         "title Foo\noptions root=/dev/sda1 verity=1",
		},
		{
			name: "multiple options lines: last line emptied gets re-populated by newArgs",
			content: "title Foo\n" +
				"options root=/dev/sda1\n" +
				"options stale=yes\n",
			argsToRemove: []string{"stale"},
			newArgs:      []string{"verity=1"},
			want: "title Foo\n" +
				"options root=/dev/sda1\n" +
				"options verity=1\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := updateBLSEntryOptions(tc.content, tc.argsToRemove, tc.newArgs)
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
			name: "missing title key is treated as a normal entry",
			files: map[string]string{
				"bad.conf": "linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1"},
			},
		},
		{
			name: "duplicate 'linux' key in a single entry produces an error",
			files: map[string]string{
				"dup-linux.conf": "title Azure Linux\n" +
					"linux /vmlinuz-6.6\n" +
					"linux /vmlinuz-6.6.also\n",
			},
			wantErrSubstr: "duplicate key (linux)",
		},
		{
			name: "duplicate 'title' key in a single entry produces an error",
			files: map[string]string{
				"dup-title.conf": "title Azure Linux\n" +
					"title Azure Linux Also\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
			},
			wantErrSubstr: "duplicate key (title)",
		},
		{
			name: "duplicate empty 'title' key in a single entry produces an error",
			files: map[string]string{
				"dup-empty-title.conf": "title\n" +
					"title\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
			},
			wantErrSubstr: "duplicate key (title)",
		},
		{
			name: "empty 'linux' value produces an error",
			files: map[string]string{
				"empty-linux.conf": "title Azure Linux\n" +
					"linux \n" +
					"options root=/dev/sda1\n",
			},
			wantErrSubstr: "'linux' key has empty value",
		},
		{
			name: "duplicate BLS entries for the same kernel produce an error",
			files: map[string]string{
				"a.conf": "title Azure Linux A\n" +
					"linux /vmlinuz-6.6\n" +
					"options root=/dev/sda1\n",
				"b.conf": "title Azure Linux B\n" +
					"linux /boot/vmlinuz-6.6\n" + // filepath.Base normalises this to the same kernel
					"options root=/dev/sda2\n",
			},
			wantErrSubstr: "duplicate BLS entries for kernel",
		},
		{
			name: "CRLF line endings are tolerated",
			files: map[string]string{
				"azl.conf": "title Azure Linux\r\n" +
					"linux /vmlinuz-6.6\r\n" +
					"options root=/dev/sda1 quiet\r\n",
			},
			wantKernels: []string{"vmlinuz-6.6"},
			wantArgsFor: map[string][]string{
				"vmlinuz-6.6": {"root=/dev/sda1", "quiet"},
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
		name    string
		value   string
		wantArg []string // expected .Arg fields, in order
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
			name:    "unquoted semicolon is passed through verbatim",
			value:   "console=ttyS0;115200 quiet",
			wantArg: []string{"console=ttyS0;115200", "quiet"},
		},
		{
			name:    "tab separates tokens",
			value:   "console=ttyS0\tquiet",
			wantArg: []string{"console=ttyS0", "quiet"},
		},
		{
			name:    "boolean arg (no equals) is a single token with empty Value",
			value:   "quiet",
			wantArg: []string{"quiet"},
		},
		{
			name:    "multiple '=' splits on the first only",
			value:   "a=b=c",
			wantArg: []string{"a=b=c"},
		},
		{
			name:    "single quote is literal, not grouping",
			value:   "rd.cmdline='foo bar' quiet",
			wantArg: []string{"rd.cmdline='foo", "bar'", "quiet"},
		},
		{
			name:    "backslash is literal (no escape)",
			value:   `foo\"bar baz" qux`,
			wantArg: []string{`foo\bar baz`, "qux"},
		},
		{
			name:    "unterminated quote absorbs to end of input as one token",
			value:   `foo="bar baz`,
			wantArg: []string{"foo=bar baz"},
		},
		{
			name:    "runs of whitespace collapse",
			value:   "   foo\t\t  bar   ",
			wantArg: []string{"foo", "bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := parseBLSOptionsValue(tc.value)
			gotArgs := make([]string, 0, len(args))
			for _, a := range args {
				gotArgs = append(gotArgs, a.Arg)
			}
			assert.Equal(t, tc.wantArg, gotArgs)
		})
	}
}

// writeTestBLSEntry creates a single AZL4-style BLS entry under {bootDir}/loader/entries and returns the entry's
// absolute path.
func writeTestBLSEntry(t *testing.T, bootDir string) string {
	entriesDir := filepath.Join(bootDir, "loader", "entries")
	err := os.MkdirAll(entriesDir, 0o755)
	assert.NoError(t, err)

	content := "title Azure Linux (6.18.31-1.5.azl4.x86_64) 4.0\n" +
		"version 6.18.31-1.5.azl4.x86_64\n" +
		"linux /boot/vmlinuz-6.18.31-1.5.azl4.x86_64\n" +
		"initrd /boot/initramfs-6.18.31-1.5.azl4.x86_64.img\n" +
		"options console=ttyS0 root=UUID=1396c02f-6cf5-438c-9c9c-fb2001079bb9 rd.shell=0\n" +
		"grub_users $grub_users\n" +
		"grub_arg --unrestricted\n" +
		"grub_class azurelinux\n"

	entryPath := filepath.Join(entriesDir, "azl4.conf")
	err = os.WriteFile(entryPath, []byte(content), 0o644)
	assert.NoError(t, err)
	return entryPath
}

func TestUpdateLiveOSBLSEntriesFullOS(t *testing.T) {
	bootDir := t.TempDir()
	entryPath := writeTestBLSEntry(t, bootDir)

	savedConfigs := &SavedConfigs{}
	savedConfigs.LiveOS.KernelCommandLine.ExtraCommandLine = []string{"rd.info"}

	err := updateLiveOSBLSEntries(bootDir, imagecustomizerapi.InitramfsImageTypeFullOS, false /*disableSELinux*/, savedConfigs)
	assert.NoError(t, err)

	got, err := os.ReadFile(entryPath)
	assert.NoError(t, err)
	gotStr := string(got)

	// Full-OS repoints initrd at the single regenerated /boot/initrd.img.
	assert.Regexp(t, `(?m)^initrd /boot/initrd\.img$`, gotStr)
	// root= is dropped so no pivot takes place, and the unrelated arg is preserved.
	assert.NotContains(t, gotStr, "root=UUID=")
	assert.Regexp(t, `(?m)^options .*console=ttyS0`, gotStr)
	// The saved extra command line is appended.
	assert.Regexp(t, `(?m)^options .* rd\.info$`, gotStr)
	// blscfg-only keys are untouched.
	assert.Contains(t, gotStr, "grub_class azurelinux")
}

func TestUpdateLiveOSBLSEntriesBootstrap(t *testing.T) {
	bootDir := t.TempDir()
	entryPath := writeTestBLSEntry(t, bootDir)

	savedConfigs := &SavedConfigs{}
	savedConfigs.LiveOS.KernelCommandLine.ExtraCommandLine = []string{"rd.shell"}

	err := updateLiveOSBLSEntries(bootDir, imagecustomizerapi.InitramfsImageTypeBootstrap, true /*disableSELinux*/, savedConfigs)
	assert.NoError(t, err)

	got, err := os.ReadFile(entryPath)
	assert.NoError(t, err)
	gotStr := string(got)

	// Bootstrap keeps the per-kernel initrd and the original root (the iso/pxe root is set later).
	assert.Regexp(t, `(?m)^initrd /boot/initramfs-6\.18\.31-1\.5\.azl4\.x86_64\.img$`, gotStr)
	assert.Contains(t, gotStr, "root=UUID=1396c02f-6cf5-438c-9c9c-fb2001079bb9")
	// The dracut live-OS args are appended.
	assert.Contains(t, gotStr, "rd.live.image")
	assert.Contains(t, gotStr, "rd.live.dir="+liveOSDir)
	assert.Contains(t, gotStr, "rd.live.squashimg="+liveOSImage)
	// SELinux is disabled.
	assert.Contains(t, gotStr, "selinux=0")
	// The saved extra command line is appended.
	assert.Regexp(t, `(?m)^options .* rd\.shell$`, gotStr)
}

func TestSetLiveOSBLSEntriesRootForIso(t *testing.T) {
	bootDir := t.TempDir()
	entryPath := writeTestBLSEntry(t, bootDir)

	err := setLiveOSBLSEntriesRoot(bootDir, "live:LABEL=foo", nil)
	assert.NoError(t, err)

	got, err := os.ReadFile(entryPath)
	assert.NoError(t, err)
	gotStr := string(got)

	assert.Contains(t, gotStr, "root=live:LABEL=foo")
	assert.NotContains(t, gotStr, "root=UUID=")
}

func TestSetBLSEntryField(t *testing.T) {
	content := "title Foo\n" +
		"linux /boot/vmlinuz-6.6\n" +
		"initrd /boot/initramfs-6.6.img\n" +
		"options root=/dev/sda1\n"

	got, err := setBLSEntryField(content, "initrd", "/boot/initrd.img")
	assert.NoError(t, err)
	assert.Regexp(t, `(?m)^initrd /boot/initrd\.img$`, got)
	// Other lines are preserved verbatim.
	assert.Contains(t, got, "linux /boot/vmlinuz-6.6\n")
	assert.Contains(t, got, "options root=/dev/sda1\n")

	// Absent key is an error rather than a silent no-op, mirroring the inline-grub path.
	_, err = setBLSEntryField(content, "efi", "/x")
	assert.Error(t, err)
}
