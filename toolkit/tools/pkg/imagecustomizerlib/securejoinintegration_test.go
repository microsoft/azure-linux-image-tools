// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// TestInjectFilesClampsSymlinkEscape verifies that inject-files cannot be redirected outside
// the target partition by a symlink on the destination path. It plants an absolute symlink
// 'escape' -> <host dir outside the image> in the rootfs, then injects a file whose
// destination traverses that symlink, and asserts the write is clamped inside the image.
func TestInjectFilesClampsSymlinkEscape(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestInjectFilesClampsSymlinkEscape")
	err := os.MkdirAll(testTempDir, 0o755)
	assert.NoError(t, err)
	defer os.RemoveAll(testTempDir)

	// Writable copy of the base image so we can plant a symlink into it.
	imageFile := filepath.Join(testTempDir, "image.raw")
	err = file.Copy(baseImage, imageFile)
	if !assert.NoError(t, err) {
		return
	}

	// A directory OUTSIDE the image that an escaping symlink points at.
	hostArea := filepath.Join(testTempDir, "host")
	err = os.MkdirAll(hostArea, 0o755)
	assert.NoError(t, err)

	mountPoints := []testutils.MountPoint{
		{PartitionNum: 3, Path: "/", FileSystemType: "ext4"},
		{PartitionNum: 2, Path: "/boot", FileSystemType: "ext4"},
		{PartitionNum: 1, Path: "/boot/efi", FileSystemType: "vfat"},
	}

	// Plant the escape symlink in the rootfs and capture the rootfs PARTUUID.
	plantBuildDir := filepath.Join(testTempDir, "plant")
	imageConnection, err := testutils.ConnectToImage(plantBuildDir, imageFile, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}

	err = os.Symlink(hostArea, filepath.Join(imageConnection.Chroot().RootDir(), "escape"))
	assert.NoError(t, err)

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err)
	rootDevPath := testutils.PartitionDevPath(imageConnection, 3)
	var rootPartUuid string
	for _, p := range partitions {
		if p.Path == rootDevPath {
			rootPartUuid = p.PartUuid
			break
		}
	}
	assert.NotEmpty(t, rootPartUuid, "could not determine rootfs PARTUUID")

	err = imageConnection.CleanClose()
	assert.NoError(t, err)

	if rootPartUuid == "" {
		return
	}

	// Build the inject-files config whose destination traverses the escape symlink.
	injectDir := filepath.Join(testTempDir, "inject-config")
	err = os.MkdirAll(injectDir, 0o755)
	assert.NoError(t, err)

	const sourceName = "payload.txt"
	err = os.WriteFile(filepath.Join(injectDir, sourceName), []byte("payload"), 0o644)
	assert.NoError(t, err)

	injectConfig := imagecustomizerapi.InjectFilesConfig{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureInjectFiles},
		InjectFiles: []imagecustomizerapi.InjectArtifactMetadata{
			{
				Partition: imagecustomizerapi.InjectFilePartition{
					MountIdType: imagecustomizerapi.MountIdentifierTypePartUuid,
					Id:          rootPartUuid,
				},
				Source:      sourceName,
				Destination: "/escape/pwned.txt",
			},
		},
	}
	injectConfigPath := filepath.Join(injectDir, "inject-files.yaml")
	err = imagecustomizerapi.MarshalYamlFile(injectConfigPath, &injectConfig)
	if !assert.NoError(t, err) {
		return
	}

	options := InjectFilesOptions{
		BuildDir:       filepath.Join(testTempDir, "inject-build"),
		InputImageFile: imageFile,
	}
	err = InjectFilesWithConfigFile(t.Context(), injectConfigPath, options)
	assert.NoError(t, err)

	// The escape must be clamped: nothing lands in the host-side directory.
	entries, err := os.ReadDir(hostArea)
	assert.NoError(t, err)
	assert.Empty(t, entries, "inject-files must not escape the partition into %s", hostArea)

	// The write still succeeds, clamped within the image. An absolute 'escape' -> hostArea
	// resolves under the mount root, so the in-image path mirrors hostArea's absolute path.
	verifyBuildDir := filepath.Join(testTempDir, "verify")
	verifyConnection, err := testutils.ConnectToImage(verifyBuildDir, imageFile, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer verifyConnection.Close()

	clampedPath := filepath.Join(verifyConnection.Chroot().RootDir(), hostArea, "pwned.txt")
	_, err = os.Stat(clampedPath)
	assert.NoError(t, err, "expected clamped inject write within the image at %s", clampedPath)
}
