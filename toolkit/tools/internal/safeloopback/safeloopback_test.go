// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package safeloopback

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoopbackCloseWithOpenFileHandle(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices")
	}

	// Create raw disk image file.
	rawDisk, err := diskutils.CreateEmptyDisk(tmpDir, "disk.raw", 4096)
	assert.NoErrorf(t, err, "create empty disk file")

	// Attach image file.
	loopback, err := NewLoopback(rawDisk)
	assert.NoErrorf(t, err, "attach disk file")
	defer loopback.Close()

	// Open a file handle to the disk.
	// If we wanted to do this properly, we could create partitions, mount those partitions, and then create a
	// file on the partition. But just opening the disk file itself is easier.
	diskFile, err := os.OpenFile(loopback.DevicePath(), os.O_RDWR, 0)
	assert.NoErrorf(t, err, "open disk file")
	defer diskFile.Close()

	// Attempt to close the loopback with the file handle open.
	// This should fail since loopback devices won't detach until they are no longer in use.
	err = loopback.CleanClose()
	assert.Error(t, err)

	// Now close the disk file.
	diskFile.Close()

	// Loopback should now detach.
	err = loopback.CleanClose()
	assert.NoError(t, err)
}

// A customized image raises its btrfs generation, and the kernel keeps that registration after the
// device is detached. A pristine copy of the same source is then refused with EEXIST unless the
// registration is retired on detach.
func TestLoopbackCloseForgetsBtrfsFilesystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices")
	}

	if _, err := exec.LookPath("mkfs.btrfs"); err != nil {
		t.Skip("mkfs.btrfs not available")
	}

	originalPath := filepath.Join(tmpDir, "original.raw")
	err := diskutils.CreateSparseDisk(originalPath, 300, 0o644)
	require.NoError(t, err)

	loopback, err := NewLoopback(originalPath)
	require.NoError(t, err)
	defer loopback.Close()

	err = shell.NewExecBuilder("mkfs.btrfs", "-q", loopback.DevicePath()).Execute()
	require.NoError(t, err)

	err = loopback.CleanClose()
	require.NoError(t, err)

	// Keep a pristine copy, then raise the original's generation by writing to it.
	copyPath := filepath.Join(tmpDir, "copy.raw")
	err = file.Copy(originalPath, copyPath)
	require.NoError(t, err)

	err = raiseBtrfsGeneration(t, originalPath)
	require.NoError(t, err)

	// The copy still carries the original, lower generation. Mounting it must succeed.
	copyLoopback, err := NewLoopback(copyPath)
	require.NoError(t, err)
	defer copyLoopback.Close()

	mountDir := filepath.Join(tmpDir, "copy-mount")
	mount, err := safemount.NewMount(copyLoopback.DevicePath(), mountDir, "btrfs", 0, "", true)
	require.NoError(t, err, "a pristine copy must mount after the modified image is detached")

	err = mount.CleanClose()
	require.NoError(t, err)

	err = copyLoopback.CleanClose()
	require.NoError(t, err)
}

// raiseBtrfsGeneration mounts the image, writes to it and detaches it, leaving the filesystem at a
// higher generation than any copy taken beforehand.
func raiseBtrfsGeneration(t *testing.T, imagePath string) error {
	t.Helper()

	loopback, err := NewLoopback(imagePath)
	if err != nil {
		return err
	}
	defer loopback.Close()

	mountDir := filepath.Join(tmpDir, "generation-mount")
	mount, err := safemount.NewMount(loopback.DevicePath(), mountDir, "btrfs", 0, "", true)
	if err != nil {
		return err
	}

	for i := range 8 {
		err = os.WriteFile(filepath.Join(mountDir, fmt.Sprintf("file-%d", i)), make([]byte, 1024*1024), 0o644)
		if err != nil {
			return err
		}

		err = shell.NewExecBuilder("sync").Execute()
		if err != nil {
			return err
		}
	}

	err = mount.CleanClose()
	if err != nil {
		return err
	}

	return loopback.CleanClose()
}
