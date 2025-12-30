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
			name:          "Standard subvolume paths without FS_TREE prefix",
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
			name:          "Subvolume paths with FS_TREE prefix (from -a flag on non-toplevel mount)",
			output:        "ID 256 gen 7 top level 5 path <FS_TREE>/root\nID 257 gen 7 top level 5 path <FS_TREE>/home",
			expectedPaths: []string{"root", "home"},
			expectedError: false,
		},
		{
			name:          "Mixed paths with and without FS_TREE prefix",
			output:        "ID 256 gen 7 top level 5 path <FS_TREE>/root\nID 257 gen 7 top level 5 path nested_snap",
			expectedPaths: []string{"root", "nested_snap"},
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
		{
			name:          "Nested subvolumes with FS_TREE prefix",
			output:        "ID 256 gen 7 top level 5 path <FS_TREE>/root\nID 257 gen 7 top level 5 path <FS_TREE>/root/var",
			expectedPaths: []string{"root", "root/var"},
			expectedError: false,
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

	found, subvolPath, err := findFstabInRoot(partitionInfo, searchDir)
	if !assert.NoError(t, err, "findFstabInRoot") {
		return
	}

	assert.True(t, found, "fstab should be found in btrfs subvolume")
	assert.Equal(t, "root", subvolPath, "subvolume path should be 'root'")
}

func TestFindFstabInBtrfsSubvolumeWithFsTreePrefix(t *testing.T) {
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

	testTmpDir := filepath.Join(tmpDir, "TestFindFstabInBtrfsSubvolumeWithFsTreePrefix")
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

	err = shell.ExecuteLive(true, "btrfs", "subvolume", "create", filepath.Join(mountDir, "root"))
	if !assert.NoError(t, err, "create 'root' subvolume") {
		btrfsMount.Close()
		return
	}

	subvolumes, err := listBtrfsSubvolumes(mountDir)
	if !assert.NoError(t, err, "list btrfs subvolumes") {
		btrfsMount.Close()
		return
	}

	assert.Contains(t, subvolumes, "root", "subvolumes should include 'root' (with <FS_TREE>/ stripped if present)")

	etcDir := filepath.Join(mountDir, "root", "etc")
	err = os.MkdirAll(etcDir, 0755)
	if !assert.NoError(t, err) {
		btrfsMount.Close()
		return
	}

	fstabContent := "# Test fstab\n"
	err = os.WriteFile(filepath.Join(etcDir, "fstab"), []byte(fstabContent), 0644)
	if !assert.NoError(t, err) {
		btrfsMount.Close()
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

	found, subvolPath, err := findFstabInRoot(partitionInfo, searchDir)
	assert.NoError(t, err)
	assert.True(t, found, "fstab should be found")
	assert.Equal(t, "root", subvolPath, "subvolume path should be 'root'")
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

	etcDir := filepath.Join(mountDir, "etc")
	err = os.MkdirAll(etcDir, 0755)
	if !assert.NoError(t, err) {
		btrfsMount.Close()
		return
	}

	fstabContent := "# Test fstab at root\n"
	err = os.WriteFile(filepath.Join(etcDir, "fstab"), []byte(fstabContent), 0644)
	if !assert.NoError(t, err) {
		btrfsMount.Close()
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

	found, subvolPath, err := findFstabInRoot(partitionInfo, searchDir)
	assert.NoError(t, err)
	assert.True(t, found, "fstab should be found at root")
	assert.Empty(t, subvolPath, "subvolume path should be empty when fstab is at root")
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

	err = shell.ExecuteLive(true, "btrfs", "subvolume", "create", filepath.Join(mountDir, "data"))
	if !assert.NoError(t, err, "create 'data' subvolume") {
		btrfsMount.Close()
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

	found, subvolPath, err := findFstabInRoot(partitionInfo, searchDir)
	assert.NoError(t, err)
	assert.False(t, found, "fstab should NOT be found")
	assert.Empty(t, subvolPath, "subvolume path should be empty when fstab is not found")
}
