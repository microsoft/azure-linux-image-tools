// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeInitramfsRegenerator records whether RegenerateInitramfs was invoked.
type fakeInitramfsRegenerator struct {
	called int
	err    error
}

func (f *fakeInitramfsRegenerator) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	f.called++
	return f.err
}

func writeFile(t *testing.T, path string) {
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
}

func TestFindKernelsMissingInitramfs(t *testing.T) {
	bootDir := t.TempDir()
	// Kernel A has an initramfs; kernel B (the swapped-in kernel) does not.
	writeFile(t, filepath.Join(bootDir, "vmlinuz-6.6.0-1.azl3"))
	writeFile(t, filepath.Join(bootDir, "initramfs-6.6.0-1.azl3.img"))
	writeFile(t, filepath.Join(bootDir, "vmlinuz-6.18.36.1-1.azl3"))

	missing, err := findKernelsMissingInitramfs(bootDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"vmlinuz-6.18.36.1-1.azl3"}, missing)
}

func TestFindKernelsMissingInitramfsNoneMissing(t *testing.T) {
	bootDir := t.TempDir()
	writeFile(t, filepath.Join(bootDir, "vmlinuz-6.6.0-1.azl3"))
	writeFile(t, filepath.Join(bootDir, "initramfs-6.6.0-1.azl3.img"))

	missing, err := findKernelsMissingInitramfs(bootDir)
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestRegenerateMissingInitramfsRegeneratesWhenMissing(t *testing.T) {
	bootDir := t.TempDir()
	writeFile(t, filepath.Join(bootDir, "vmlinuz-6.18.36.1-1.azl3"))

	regen := &fakeInitramfsRegenerator{}
	err := regenerateMissingInitramfs(context.Background(), bootDir, nil, regen)
	require.NoError(t, err)
	assert.Equal(t, 1, regen.called, "RegenerateInitramfs should be called when a kernel lacks an initramfs")
}

func TestRegenerateMissingInitramfsNoOpWhenPresent(t *testing.T) {
	bootDir := t.TempDir()
	writeFile(t, filepath.Join(bootDir, "vmlinuz-6.6.0-1.azl3"))
	writeFile(t, filepath.Join(bootDir, "initramfs-6.6.0-1.azl3.img"))

	regen := &fakeInitramfsRegenerator{}
	err := regenerateMissingInitramfs(context.Background(), bootDir, nil, regen)
	require.NoError(t, err)
	assert.Equal(t, 0, regen.called, "RegenerateInitramfs should not be called when all kernels have an initramfs")
}
