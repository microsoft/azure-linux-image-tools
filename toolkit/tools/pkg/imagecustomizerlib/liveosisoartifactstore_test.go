// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestIsoFilesStore(artifactsDir string) *IsoFilesStore {
	return &IsoFilesStore{
		artifactsDir:    artifactsDir,
		additionalFiles: make(map[string]string),
		kernelBootFiles: make(map[string]*KernelBootFiles),
		kdumpBootFiles:  make(map[string]*KdumpBootFiles),
	}
}

// Files directly under boot/ that carry the kernel version are captured as kernel-specific (the kernel binary and its
// companion config/System.map/symvers files), so they are not scheduled as additional files.
func TestStoreIfKernelSpecificFileCapturesBootFiles(t *testing.T) {
	const kernelVersion = "6.18.5-1.azl4.x86_64"
	artifactsDir := t.TempDir()
	bootDir := filepath.Join(artifactsDir, "boot")
	filesStore := newTestIsoFilesStore(artifactsDir)

	vmlinuzPath := filepath.Join(bootDir, vmLinuzPrefix+kernelVersion)
	scheduleAdditional := storeIfKernelSpecificFile(filesStore, vmlinuzPath, []string{kernelVersion})

	assert.False(t, scheduleAdditional)
	if assert.Contains(t, filesStore.kernelBootFiles, kernelVersion) {
		assert.Equal(t, vmlinuzPath, filesStore.kernelBootFiles[kernelVersion].vmlinuzPath)
	}

	configPath := filepath.Join(bootDir, "config-"+kernelVersion)
	scheduleAdditional = storeIfKernelSpecificFile(filesStore, configPath, []string{kernelVersion})

	assert.False(t, scheduleAdditional)
	if assert.Contains(t, filesStore.kernelBootFiles, kernelVersion) {
		assert.Contains(t, filesStore.kernelBootFiles[kernelVersion].otherFiles, configPath)
	}
}

// A Boot Loader Specification entry lives under boot/loader/entries rather than directly under boot/, even though its
// name contains the kernel version. It must not be captured as a kernel file; it stays an additional file so its
// subpath is preserved when staged, instead of being flattened into boot/.
func TestStoreIfKernelSpecificFileLeavesBlsEntriesAsAdditionalFiles(t *testing.T) {
	const kernelVersion = "6.18.5-1.azl4.x86_64"
	artifactsDir := t.TempDir()
	filesStore := newTestIsoFilesStore(artifactsDir)

	blsEntryPath := filepath.Join(artifactsDir, "boot", "loader", "entries", "abc123-"+kernelVersion+".conf")
	scheduleAdditional := storeIfKernelSpecificFile(filesStore, blsEntryPath, []string{kernelVersion})

	assert.True(t, scheduleAdditional)
	assert.NotContains(t, filesStore.kernelBootFiles, kernelVersion)
}
