// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAclVerityUsrHash(t *testing.T) {
	tests := []struct {
		name         string
		cmdline      string
		expectedHash string
		expectedOk   bool
	}{
		{
			name:         "usrhash present",
			cmdline:      "console=tty0 usrhash=abc123 root=/dev/sda",
			expectedHash: "abc123",
			expectedOk:   true,
		},
		{
			name:         "systemd.verity_usr_hash present",
			cmdline:      "console=tty0 systemd.verity_usr_hash=def456 root=/dev/sda",
			expectedHash: "def456",
			expectedOk:   true,
		},
		{
			name:         "no verity hash arg",
			cmdline:      "console=tty0 root=/dev/sda",
			expectedHash: "",
			expectedOk:   false,
		},
		{
			name:         "empty cmdline",
			cmdline:      "",
			expectedHash: "",
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, ok, err := extractAclVerityUsrHash(tt.cmdline)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOk, ok)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestAclRewriteVerityHashArgs(t *testing.T) {
	tests := []struct {
		name            string
		cmdline         string
		newHash         string
		expectedCmdline string
		expectedChanged bool
	}{
		{
			name: "rewrites both hash args, leaves other verity args untouched",
			cmdline: "console=tty0 systemd.verity_usr_data=/dev/disk/by-partuuid/aaa " +
				"systemd.verity_usr_hash=oldhash systemd.verity_usr_options=panic-on-corruption usrhash=oldhash",
			newHash: "newhash",
			expectedCmdline: "console=tty0 systemd.verity_usr_data=/dev/disk/by-partuuid/aaa " +
				"systemd.verity_usr_hash=newhash systemd.verity_usr_options=panic-on-corruption usrhash=newhash",
			expectedChanged: true,
		},
		{
			name:            "already up to date is a no-op",
			cmdline:         "usrhash=samehash systemd.verity_usr_hash=samehash",
			newHash:         "samehash",
			expectedCmdline: "usrhash=samehash systemd.verity_usr_hash=samehash",
			expectedChanged: false,
		},
		{
			name:            "no hash args present leaves cmdline untouched",
			cmdline:         "console=tty0 root=/dev/sda",
			newHash:         "newhash",
			expectedCmdline: "console=tty0 root=/dev/sda",
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdline, changed, err := aclRewriteVerityHashArgs(tt.cmdline, tt.newHash)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedChanged, changed)
			assert.Equal(t, tt.expectedCmdline, cmdline)
		})
	}
}

func TestExtractConsistentAclVerityUsrHash(t *testing.T) {
	tests := []struct {
		name              string
		kernelInfo        map[string]UkiKernelInfo
		expectedHash      string
		expectedErrSubstr string
	}{
		{
			name: "single kernel with hash",
			kernelInfo: map[string]UkiKernelInfo{
				"6.6.92.2-2.azl3": {Cmdline: "console=tty0 usrhash=abc123"},
			},
			expectedHash: "abc123",
		},
		{
			name: "multiple kernels agree",
			kernelInfo: map[string]UkiKernelInfo{
				"6.6.92.2-2.azl3": {Cmdline: "console=tty0 usrhash=abc123"},
				"6.6.92.3-1.azl3": {Cmdline: "console=ttyS0 usrhash=abc123"},
			},
			expectedHash: "abc123",
		},
		{
			name: "no kernel has a verity hash",
			kernelInfo: map[string]UkiKernelInfo{
				"6.6.92.2-2.azl3": {Cmdline: "console=tty0 root=/dev/sda"},
			},
			expectedHash: "",
		},
		{
			name: "kernels disagree",
			kernelInfo: map[string]UkiKernelInfo{
				"6.6.92.2-2.azl3": {Cmdline: "console=tty0 usrhash=abc123"},
				"6.6.92.3-1.azl3": {Cmdline: "console=ttyS0 usrhash=def456"},
			},
			expectedErrSubstr: "inconsistent /usr dm-verity root hashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := extractConsistentAclVerityUsrHash(tt.kernelInfo)
			if tt.expectedErrSubstr != "" {
				assert.ErrorContains(t, err, tt.expectedErrSubstr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestAclUpdateVerityAddonTemplatesNoOp(t *testing.T) {
	testTempDir := t.TempDir()

	// No-op: empty hash means there is nothing to propagate, regardless of whether the template
	// directory exists.
	err := aclUpdateVerityAddonTemplates(testTempDir, testTempDir, filepath.Join(testTempDir, "stub.efi"), "")
	assert.NoError(t, err)

	// No-op: template directory doesn't exist (non-ACL image, or an ACL image predating the
	// per-slot verity addon template contract).
	err = aclUpdateVerityAddonTemplates(testTempDir, testTempDir, filepath.Join(testTempDir, "stub.efi"), "somehash")
	assert.NoError(t, err)

	// No-op (per-file skip, not an error): template directory exists but is empty.
	templateDir := filepath.Join(testTempDir, aclVerityAddonTemplatesDir)
	err = os.MkdirAll(templateDir, os.ModePerm)
	assert.NoError(t, err)

	err = aclUpdateVerityAddonTemplates(testTempDir, testTempDir, filepath.Join(testTempDir, "stub.efi"), "somehash")
	assert.NoError(t, err)
}
