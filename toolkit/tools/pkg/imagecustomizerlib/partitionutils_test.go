// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestParseBtrfsSubvolumeOutput(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedPaths []string
		expectedError bool
	}{
		{
			name:          "Standard subvolume paths",
			output:        "ID 256 gen 7 top level 5 path root\nID 257 gen 7 top level 5 path home",
			expectedPaths: []string{"root", "home"},
			expectedError: false,
		},
		{
			name:          "Subvolume paths with @ prefix (Ubuntu/Arch style)",
			output:        "ID 256 gen 7 top level 5 path @\nID 257 gen 7 top level 5 path @home",
			expectedPaths: []string{"@", "@home"},
			expectedError: false,
		},
		{
			name:          "Nested subvolumes",
			output:        "ID 256 gen 7 top level 5 path root\nID 257 gen 7 top level 5 path root/var",
			expectedPaths: []string{"root", "root/var"},
			expectedError: false,
		},
		{
			name: "Deeply nested subvolumes",
			output: "ID 256 gen 7 top level 5 path root\n" +
				"ID 257 gen 7 top level 5 path home\n" +
				"ID 258 gen 7 top level 256 path root/var\n" +
				"ID 259 gen 7 top level 258 path root/var/log",
			expectedPaths: []string{"root", "home", "root/var", "root/var/log"},
			expectedError: false,
		},
		{
			name:          "Empty output",
			output:        "",
			expectedPaths: nil,
			expectedError: false,
		},
		{
			name:          "Only whitespace",
			output:        "   \n   \n   ",
			expectedPaths: nil,
			expectedError: false,
		},
		{
			name:          "Invalid output format",
			output:        "invalid line without proper format",
			expectedPaths: nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := parseBtrfsSubvolumeListOutput(tt.output)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPaths, paths)
			}
		})
	}
}

func TestFindFstabInBtrfsSubvolume(t *testing.T) {
	// This test requires root privileges and btrfs-progs
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices and mounts")
	}

	btrfsExists, err := file.CommandExists("btrfs")
	if err != nil || !btrfsExists {
		t.Skip("The 'btrfs' command is not available")
	}

	mkfsBtrfsExists, err := file.CommandExists("mkfs.btrfs")
	if err != nil || !mkfsBtrfsExists {
		t.Skip("The 'mkfs.btrfs' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestFindFstabInBtrfsSubvolume")
	err = os.MkdirAll(testTmpDir, 0755)
	if !assert.NoError(t, err) {
		return
	}
	defer os.RemoveAll(testTmpDir)

	imageFile := filepath.Join(testTmpDir, "test.raw")
	err = shell.ExecuteLive(true, "dd", "if=/dev/zero", "of="+imageFile, "bs=1M", "count=200")
	if !assert.NoError(t, err, "create raw disk image") {
		return
	}

	loopback, err := safeloopback.NewLoopback(imageFile)
	if !assert.NoError(t, err, "setup loopback device") {
		return
	}
	defer loopback.Close()

	err = shell.ExecuteLive(true, "mkfs.btrfs", "-f", loopback.DevicePath())
	if !assert.NoError(t, err, "create btrfs filesystem") {
		return
	}

	mountDir := filepath.Join(testTmpDir, "mount")
	err = os.MkdirAll(mountDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(loopback.DevicePath(), mountDir, "btrfs", 0, "", false)
	if !assert.NoError(t, err, "mount btrfs filesystem") {
		return
	}
	defer btrfsMount.Close()

	err = shell.ExecuteLive(true, "btrfs", "subvolume", "create", filepath.Join(mountDir, "root"))
	if !assert.NoError(t, err, "create 'root' subvolume") {
		return
	}

	err = shell.ExecuteLive(true, "btrfs", "subvolume", "create", filepath.Join(mountDir, "home"))
	if !assert.NoError(t, err, "create 'home' subvolume") {
		return
	}

	etcDir := filepath.Join(mountDir, "root", "etc")
	err = os.MkdirAll(etcDir, 0755)
	if !assert.NoError(t, err, "create etc directory in subvolume") {
		return
	}

	fstabContent := `# /etc/fstab
UUID=test-uuid  /       btrfs   subvol=/root,defaults   0 0
UUID=test-uuid  /home   btrfs   subvol=/home,defaults   0 0
`
	fstabPath := filepath.Join(etcDir, "fstab")
	err = os.WriteFile(fstabPath, []byte(fstabContent), 0644)
	if !assert.NoError(t, err, "write fstab file") {
		return
	}

	err = btrfsMount.CleanClose()
	if !assert.NoError(t, err, "unmount btrfs filesystem") {
		return
	}

	partitionInfo := diskutils.PartitionInfo{
		Path:           loopback.DevicePath(),
		FileSystemType: "btrfs",
		Type:           "part",
	}

	searchDir := filepath.Join(testTmpDir, "search")
	err = os.MkdirAll(searchDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	fstabPaths, err := findFstabInRoot(partitionInfo, searchDir)
	if !assert.NoError(t, err, "findFstabInRoot") {
		return
	}

	assert.Len(t, fstabPaths, 1, "fstab should be found in btrfs subvolume")
	assert.Equal(t, "root", fstabPaths[0], "subvolume path should be 'root'")
}

func TestFindFstabAtBtrfsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices and mounts")
	}

	btrfsExists, err := file.CommandExists("btrfs")
	if err != nil || !btrfsExists {
		t.Skip("The 'btrfs' command is not available")
	}

	mkfsBtrfsExists, err := file.CommandExists("mkfs.btrfs")
	if err != nil || !mkfsBtrfsExists {
		t.Skip("The 'mkfs.btrfs' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestFindFstabAtBtrfsRoot")
	err = os.MkdirAll(testTmpDir, 0755)
	if !assert.NoError(t, err) {
		return
	}
	defer os.RemoveAll(testTmpDir)

	imageFile := filepath.Join(testTmpDir, "test.raw")
	err = shell.ExecuteLive(true, "dd", "if=/dev/zero", "of="+imageFile, "bs=1M", "count=200")
	if !assert.NoError(t, err, "create raw disk image") {
		return
	}

	loopback, err := safeloopback.NewLoopback(imageFile)
	if !assert.NoError(t, err, "setup loopback device") {
		return
	}
	defer loopback.Close()

	err = shell.ExecuteLive(true, "mkfs.btrfs", "-f", loopback.DevicePath())
	if !assert.NoError(t, err, "create btrfs filesystem") {
		return
	}

	mountDir := filepath.Join(testTmpDir, "mount")
	err = os.MkdirAll(mountDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(loopback.DevicePath(), mountDir, "btrfs", 0, "", false)
	if !assert.NoError(t, err, "mount btrfs filesystem") {
		return
	}
	defer btrfsMount.Close()

	etcDir := filepath.Join(mountDir, "etc")
	err = os.MkdirAll(etcDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	fstabContent := "# Test fstab at root\n"
	err = os.WriteFile(filepath.Join(etcDir, "fstab"), []byte(fstabContent), 0644)
	if !assert.NoError(t, err) {
		return
	}

	err = btrfsMount.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	partitionInfo := diskutils.PartitionInfo{
		Path:           loopback.DevicePath(),
		FileSystemType: "btrfs",
		Type:           "part",
	}

	searchDir := filepath.Join(testTmpDir, "search")
	err = os.MkdirAll(searchDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	fstabPaths, err := findFstabInRoot(partitionInfo, searchDir)
	assert.NoError(t, err)
	assert.Len(t, fstabPaths, 1, "fstab should be found at root")
	assert.Empty(t, fstabPaths[0], "subvolume path should be empty when fstab is at root")
}

func TestFindFstabNotFoundInBtrfs(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses loopback devices and mounts")
	}

	btrfsExists, err := file.CommandExists("btrfs")
	if err != nil || !btrfsExists {
		t.Skip("The 'btrfs' command is not available")
	}

	mkfsBtrfsExists, err := file.CommandExists("mkfs.btrfs")
	if err != nil || !mkfsBtrfsExists {
		t.Skip("The 'mkfs.btrfs' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestFindFstabNotFoundInBtrfs")
	err = os.MkdirAll(testTmpDir, 0755)
	if !assert.NoError(t, err) {
		return
	}
	defer os.RemoveAll(testTmpDir)

	imageFile := filepath.Join(testTmpDir, "test.raw")
	err = shell.ExecuteLive(true, "dd", "if=/dev/zero", "of="+imageFile, "bs=1M", "count=200")
	if !assert.NoError(t, err, "create raw disk image") {
		return
	}

	loopback, err := safeloopback.NewLoopback(imageFile)
	if !assert.NoError(t, err, "setup loopback device") {
		return
	}
	defer loopback.Close()

	err = shell.ExecuteLive(true, "mkfs.btrfs", "-f", loopback.DevicePath())
	if !assert.NoError(t, err, "create btrfs filesystem") {
		return
	}

	mountDir := filepath.Join(testTmpDir, "mount")
	err = os.MkdirAll(mountDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(loopback.DevicePath(), mountDir, "btrfs", 0, "", false)
	if !assert.NoError(t, err, "mount btrfs filesystem") {
		return
	}
	defer btrfsMount.Close()

	err = shell.ExecuteLive(true, "btrfs", "subvolume", "create", filepath.Join(mountDir, "data"))
	if !assert.NoError(t, err, "create 'data' subvolume") {
		return
	}

	err = btrfsMount.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	partitionInfo := diskutils.PartitionInfo{
		Path:           loopback.DevicePath(),
		FileSystemType: "btrfs",
		Type:           "part",
	}

	searchDir := filepath.Join(testTmpDir, "search")
	err = os.MkdirAll(searchDir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	fstabPaths, err := findFstabInRoot(partitionInfo, searchDir)
	assert.NoError(t, err)
	assert.Empty(t, fstabPaths, "fstab should NOT be found")
}
