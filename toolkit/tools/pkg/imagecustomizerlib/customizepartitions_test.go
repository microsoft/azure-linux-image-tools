// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/kernelversion"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImagePartitions(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsToEfi(t, "TestCustomizeImagePartitions"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsToEfi(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyEfiPartitionsImage(t, outImageFilePath, baseImageInfo, buildDir)
}

func verifyEfiPartitionsImage(t *testing.T, outImageFilePath string, baseImageInfo testBaseImageInfo, buildDir string) {
	// Check output file type.
	checkFileType(t, outImageFilePath, "raw")

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "xfs",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   4,
			Path:           "/var",
			FileSystemType: "xfs",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	if assert.NoError(t, err, "read partition table") {
		assert.Equal(t, "", partitions[1].PartLabel)
		assert.Equal(t, "", partitions[2].PartLabel)
		assert.Equal(t, "rootfs", partitions[3].PartLabel)
		assert.Equal(t, "", partitions[4].PartLabel)
	}

	// Check for key files/directories on the partitions.
	_, err = os.Stat(filepath.Join(imageConnection.Chroot().RootDir(), "/usr/bin/bash"))
	assert.NoError(t, err, "check for /usr/bin/bash")

	_, err = os.Stat(filepath.Join(imageConnection.Chroot().RootDir(), "/var/log"))
	assert.NoError(t, err, "check for /var/log")

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, mountPoints, partitions)
	verifyBootloaderConfig(t, imageConnection, "console=tty0 console=ttyS0",
		partitions[mountPoints[1].PartitionNum],
		partitions[mountPoints[0].PartitionNum],
		baseImageInfo)

	// Check the partition types.
	assert.Equal(t, "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", partitions[1].PartitionTypeUuid) // esp
	assert.Equal(t, "bc13c2ff-59e6-4262-a352-b275fd6f7172", partitions[2].PartitionTypeUuid) // xbootldr
	assert.Equal(t, "4d21b016-b534-45c2-a9fb-5c16e091fd2d", partitions[4].PartitionTypeUuid) // var

	switch runtime.GOARCH {
	case "amd64":
		assert.Equal(t, "4f68bce3-e8cd-4db1-96e7-fbcaf984b709", partitions[3].PartitionTypeUuid) // root (x64)
	case "arm64":
		assert.Equal(t, "b921b045-1df0-41c3-af44-4c6f280d3fae", partitions[3].PartitionTypeUuid) // root (arm64)
	}

	// Check the partition sizes.
	assert.Equal(t, uint64(8*diskutils.MiB), partitions[1].SizeInBytes)
	assert.Equal(t, uint64(99*diskutils.MiB), partitions[2].SizeInBytes)
	assert.Equal(t, uint64(1940*diskutils.MiB), partitions[3].SizeInBytes)
	assert.Equal(t, uint64(2047*diskutils.MiB), partitions[4].SizeInBytes)

	// Check filesystem features
	hostKernelVersion, err := kernelversion.GetBuildHostKernelVersion()
	assert.NoError(t, err)

	switch baseImageInfo.Version {
	case baseImageVersionAzl2:
		verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "sparse", hostKernelVersion.Ge([]int{4, 10}))
		verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "nrext64", false)

	case baseImageVersionAzl3:
		verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "sparse", hostKernelVersion.Ge([]int{4, 10}))
		verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "nrext64", hostKernelVersion.Ge([]int{5, 19}))
	}
}

func TestCustomizeImagePartitionsSizeOnly(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePartitionsSizeOnly")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-size-only-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Check output file type.
	checkFileType(t, outImageFilePath, "raw")

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
		{
			PartitionNum:   3,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check for key files/directories on the partitions.
	_, err = os.Stat(filepath.Join(imageConnection.Chroot().RootDir(), "/usr/bin/bash"))
	assert.NoError(t, err, "check for /usr/bin/bash")

	_, err = os.Stat(filepath.Join(imageConnection.Chroot().RootDir(), "/var/log"))
	assert.NoError(t, err, "check for /var/log")

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, mountPoints, partitions)
	verifyBootloaderConfig(t, imageConnection, "",
		partitions[mountPoints[0].PartitionNum],
		partitions[mountPoints[0].PartitionNum],
		baseImageInfo)

	// Check the partition types.
	assert.Equal(t, "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", partitions[1].PartitionTypeUuid) // esp
	assert.Equal(t, "0fc63daf-8483-4772-8e79-3d69d8477de4", partitions[2].PartitionTypeUuid) // linux generic
	assert.Equal(t, "0fc63daf-8483-4772-8e79-3d69d8477de4", partitions[3].PartitionTypeUuid) // linux generic

	// Check the partition sizes.
	assert.Equal(t, uint64(8*diskutils.MiB), partitions[1].SizeInBytes)
	assert.Equal(t, uint64(2*diskutils.GiB), partitions[2].SizeInBytes)
	assert.Equal(t, uint64(2*diskutils.GiB), partitions[3].SizeInBytes)
}

func TestCustomizeImagePartitionsLegacy(t *testing.T) {
	// Skip this test on arm64 because the legacy bootloader is not supported.
	if runtime.GOARCH == "arm64" {
		t.Skip("Skipping legacy test for arm64")
	}

	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsLegacy(t, "TestCustomizeImagePartitionsLegacy"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsLegacy(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	legacybootConfigFile := filepath.Join(testDir, "legacyboot-config.yaml")
	efiConfigFile := filepath.Join(testDir, "partitions-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Convert to legacy image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, legacybootConfigFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyLegacyBootImage(t, outImageFilePath, baseImageInfo, buildDir)

	// Recustomize legacy image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, legacybootConfigFile, outImageFilePath, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyLegacyBootImage(t, outImageFilePath, baseImageInfo, buildDir)

	// Convert back to EFI image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, efiConfigFile, outImageFilePath, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyEfiPartitionsImage(t, outImageFilePath, baseImageInfo, buildDir)
}

func verifyLegacyBootImage(t *testing.T, outImageFilePath string, baseImageInfo testBaseImageInfo, buildDir string) {
	// Check output file type.
	checkFileType(t, outImageFilePath, "raw")

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, /*includeDefaultMounts*/
		azureLinuxCoreLegacyMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, azureLinuxCoreLegacyMountPoints, partitions)
	verifyBootGrubCfg(t, imageConnection, "",
		partitions[azureLinuxCoreLegacyMountPoints[0].PartitionNum],
		partitions[azureLinuxCoreLegacyMountPoints[0].PartitionNum],
		baseImageInfo)

	// Check the partition types.
	assert.Equal(t, "21686148-6449-6e6f-744e-656564454649", partitions[1].PartitionTypeUuid) // BIOS boot
	assert.Equal(t, "0fc63daf-8483-4772-8e79-3d69d8477de4", partitions[2].PartitionTypeUuid) // linux generic

	// Check the partition sizes.
	assert.Equal(t, uint64(8*diskutils.MiB), partitions[1].SizeInBytes)
	assert.Equal(t, uint64(4086*diskutils.MiB), partitions[2].SizeInBytes)
}

func TestCustomizeImageKernelCommandLine(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageKernelCommandLineHelper(t, "TestCustomizeImageKernelCommandLine"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageKernelCommandLineHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "extracommandline-config.yaml")
	outImageFilePath := filepath.Join(buildDir, "image.qcow2")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check that the extraCommandLine was added to the grub.cfg file.
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, "linux.* console=tty0 console=ttyS0 ", grubCfgContents)
}

func TestCustomizeImageNewUUIDs(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageNewUUIDsHelper(t, "TestCustomizeImageNewUUIDs"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageNewUUIDsHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "newpartitionsuuids-config.yaml")
	tempRawBaseImage := filepath.Join(testTmpDir, "baseImage.raw")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := os.MkdirAll(buildDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	// Get the partitions from the base image.
	err = shell.ExecuteLiveWithErr(1, "qemu-img", "convert", "-O", "raw", baseImage, tempRawBaseImage)
	if !assert.NoError(t, err) {
		return
	}

	baseImageLoopback, err := safeloopback.NewLoopback(tempRawBaseImage)
	if !assert.NoError(t, err) {
		return
	}
	defer baseImageLoopback.Close()

	baseImagePartitions, err := getDiskPartitionsMap(baseImageLoopback.DevicePath())
	if !assert.NoError(t, err, "get base image partitions") {
		return
	}

	err = baseImageLoopback.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	os.Remove(tempRawBaseImage)

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	newImagePartitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	if !assert.NoError(t, err, "get customized image partitions") {
		return
	}

	// Ensure the partition UUIDs have been changed.
	if assert.Equal(t, len(baseImagePartitions), len(newImagePartitions)) {
		for partitionNum := range baseImagePartitions {
			baseImagePartition := baseImagePartitions[partitionNum]
			newImagePartition := newImagePartitions[partitionNum]

			if baseImagePartition.Type != "part" {
				continue
			}

			assert.Equalf(t, baseImagePartition.FileSystemType, newImagePartition.FileSystemType, "[%d] filesystem type didn't change", partitionNum)
			assert.NotEqualf(t, baseImagePartition.PartUuid, newImagePartition.PartUuid, "[%d] partition UUID did change", partitionNum)
			assert.NotEqual(t, baseImagePartition.Uuid, newImagePartition.Uuid, "[%d] filesystem UUID did change", partitionNum)
		}
	}

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, azureLinuxCoreEfiMountPoints, newImagePartitions)
	verifyBootloaderConfig(t, imageConnection, "",
		newImagePartitions[azureLinuxCoreEfiMountPoints[0].PartitionNum],
		newImagePartitions[azureLinuxCoreEfiMountPoints[0].PartitionNum],
		baseImageInfo)
}

func TestCustomizeImagePartitionsXfsBoot(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsXfsBootHelper(t, "TestCustomizeImagePartitionsXfsBoot"+baseImageInfo.Name,
				baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsXfsBootHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-xfs-boot.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "xfs",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "read partition table")

	// Check filesystem features
	hostKernelVersion, err := kernelversion.GetBuildHostKernelVersion()
	assert.NoError(t, err)

	verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "sparse", hostKernelVersion.Ge([]int{4, 10}))

	// The /boot directory is on an XFS partition.
	// Hence 'nrext64' should be disabled since GRUB 2.06 doesn't support it.
	verifyXfsFeature(t, partitions[mountPoints[0].PartitionNum].Path, "nrext64", false)
}

func TestCustomizeImagePartitionsBtrfsBoot(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsBtrfsBootHelper(t, "TestCustomizeImagePartitionsBtrfsBoot"+baseImageInfo.Name,
				baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsBtrfsBootHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-btrfs-boot.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	loopback, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer loopback.Close()

	btrfsPartitionPath := fmt.Sprintf("%sp2", loopback.DevicePath())

	btrfsMountDir := filepath.Join(testTmpDir, "btrfsmount")
	err = os.MkdirAll(btrfsMountDir, 0o755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(btrfsPartitionPath, btrfsMountDir, "btrfs", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer btrfsMount.Close()

	verifyBtrfsSubvolumes(t, btrfsMountDir, nil)
	verifyBtrfsQuotasDisabled(t, btrfsMountDir)
}

func TestCustomizeImagePartitionsBtrfsSubvolumesBasic(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsBtrfsSubvolumesBasicHelper(t,
				"TestCustomizeImagePartitionsBtrfsSubvolumesBasic"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsBtrfsSubvolumesBasicHelper(t *testing.T, testName string,
	baseImageInfo testBaseImageInfo,
) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-btrfs-subvolumes-basic.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	loopback, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer loopback.Close()

	btrfsPartitionPath := fmt.Sprintf("%sp2", loopback.DevicePath())

	btrfsMountDir := filepath.Join(testTmpDir, "btrfsmount")
	err = os.MkdirAll(btrfsMountDir, 0o755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(btrfsPartitionPath, btrfsMountDir, "btrfs", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer btrfsMount.Close()

	expectedSubvolumes := []string{"root", "home"}
	verifyBtrfsSubvolumes(t, btrfsMountDir, expectedSubvolumes)

	fstabPath := filepath.Join(btrfsMountDir, "root", "etc", "fstab")
	expectedOptions := map[string]string{
		"/":     "subvol=/root",
		"/home": "subvol=/home",
	}
	verifyBtrfsFstabEntries(t, fstabPath, expectedOptions)

	verifyBtrfsQuotasDisabled(t, btrfsMountDir)
}

func TestCustomizeImagePartitionsBtrfsSubvolumesNested(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsBtrfsSubvolumesNestedHelper(t,
				"TestCustomizeImagePartitionsBtrfsSubvolumesNested"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsBtrfsSubvolumesNestedHelper(t *testing.T, testName string,
	baseImageInfo testBaseImageInfo,
) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-btrfs-subvolumes-nested.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	loopback, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer loopback.Close()

	btrfsPartitionPath := fmt.Sprintf("%sp2", loopback.DevicePath())

	btrfsMountDir := filepath.Join(testTmpDir, "btrfsmount")
	err = os.MkdirAll(btrfsMountDir, 0o755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(btrfsPartitionPath, btrfsMountDir, "btrfs", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer btrfsMount.Close()

	expectedSubvolumes := []string{"root", "home", "root/var", "root/var/log", "snapshots"}
	verifyBtrfsSubvolumes(t, btrfsMountDir, expectedSubvolumes)

	fstabPath := filepath.Join(btrfsMountDir, "root", "etc", "fstab")
	expectedOptions := map[string]string{
		"/":        "subvol=/root",
		"/home":    "subvol=/home",
		"/var":     "subvol=/root/var",
		"/var/log": "subvol=/root/var/log,noatime",
	}
	verifyBtrfsFstabEntries(t, fstabPath, expectedOptions)

	expectedQuotas := []btrfsExpectedQuota{
		{"home", 500 * diskutils.MiB, 0},
		{"root/var/log", 100 * diskutils.MiB, 50 * diskutils.MiB},
	}
	verifyBtrfsQuotas(t, btrfsMountDir, expectedQuotas)
}

func TestCustomizeImagePartitionsBtrfsUnmounted(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsBtrfsUnmountedHelper(t,
				"TestCustomizeImagePartitionsBtrfsUnmounted"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsBtrfsUnmountedHelper(t *testing.T, testName string,
	baseImageInfo testBaseImageInfo,
) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-btrfs-unmounted.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	loopback, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer loopback.Close()

	rootPartitionPath := fmt.Sprintf("%sp2", loopback.DevicePath())

	rootMountDir := filepath.Join(testTmpDir, "rootmount")
	err = os.MkdirAll(rootMountDir, 0o755)
	if !assert.NoError(t, err) {
		return
	}

	rootMount, err := safemount.NewMount(rootPartitionPath, rootMountDir, "ext4", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer rootMount.Close()

	fstabPath := filepath.Join(rootMountDir, "etc", "fstab")
	verifyBtrfsFstabEntries(t, fstabPath, nil)

	btrfsPartitionPath := fmt.Sprintf("%sp3", loopback.DevicePath())

	btrfsMountDir := filepath.Join(testTmpDir, "btrfsmount")
	err = os.MkdirAll(btrfsMountDir, 0o755)
	if !assert.NoError(t, err) {
		return
	}

	btrfsMount, err := safemount.NewMount(btrfsPartitionPath, btrfsMountDir, "btrfs", 0, "", true)
	if !assert.NoError(t, err, "should be able to mount btrfs partition") {
		return
	}
	defer btrfsMount.Close()

	_, err = os.ReadDir(btrfsMountDir)
	assert.NoError(t, err, "should be able to read btrfs filesystem")

	verifyBtrfsSubvolumes(t, btrfsMountDir, nil)
	verifyBtrfsQuotasDisabled(t, btrfsMountDir)
}

func getFilteredFstabEntries(t *testing.T, imageConnection *imageconnection.ImageConnection) []diskutils.FstabEntry {
	fstabPath := filepath.Join(imageConnection.Chroot().RootDir(), "/etc/fstab")
	fstabEntries, err := diskutils.ReadFstabFile(fstabPath)
	if !assert.NoError(t, err, "read /etc/fstab") {
		return nil
	}

	filteredFstabEntries := filterOutSpecialPartitions(fstabEntries)
	return filteredFstabEntries
}

func verifyFstabEntries(t *testing.T, imageConnection *imageconnection.ImageConnection, mountPoints []testutils.MountPoint,
	partitions map[int]diskutils.PartitionInfo,
) {
	filteredFstabEntries := getFilteredFstabEntries(t, imageConnection)
	if filteredFstabEntries == nil {
		return
	}

	if !assert.Equalf(t, len(mountPoints), len(filteredFstabEntries), "/etc/fstab entries count: %v", filteredFstabEntries) {
		return
	}

	for i := range mountPoints {
		mountPoint := mountPoints[i]
		fstabEntry := filteredFstabEntries[i]
		partition := partitions[mountPoint.PartitionNum]

		assert.Equalf(t, mountPoint.FileSystemType, fstabEntry.FsType, "fstab [%d]: file system type", i)
		assert.Equalf(t, mountPoint.Path, fstabEntry.Target, "fstab [%d]: target path", i)

		expectedSource := fmt.Sprintf("PARTUUID=%s", partition.PartUuid)
		assert.Equalf(t, expectedSource, fstabEntry.Source, "fstab [%d]: source", i)
	}
}

func verifyBootloaderConfig(t *testing.T, imageConnection *imageconnection.ImageConnection, extraCommandLineArgs string,
	bootInfo diskutils.PartitionInfo, rootfsInfo diskutils.PartitionInfo, baseImageInfo testBaseImageInfo,
) {
	verifyEspGrubCfg(t, imageConnection, bootInfo.Uuid)
	verifyBootGrubCfg(t, imageConnection, extraCommandLineArgs, bootInfo, rootfsInfo, baseImageInfo)
}

func verifyEspGrubCfg(t *testing.T, imageConnection *imageconnection.ImageConnection, bootUuid string) {
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/efi/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	if !assert.NoError(t, err, "read ESP grub.cfg file") {
		return
	}

	assert.Regexp(t, fmt.Sprintf("(?m)^search -n -u %s -s$", regexp.QuoteMeta(bootUuid)), grubCfgContents)
}

func verifyBootGrubCfg(t *testing.T, imageConnection *imageconnection.ImageConnection, extraCommandLineArgs string,
	bootInfo diskutils.PartitionInfo, rootfsInfo diskutils.PartitionInfo,
	baseImageInfo testBaseImageInfo,
) {
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	if !assert.NoError(t, err, "read boot grub.cfg file") {
		return
	}

	switch baseImageInfo.Version {
	case baseImageVersionAzl2:
		assert.Regexp(t, fmt.Sprintf(`(?m)^search -n -u %s -s$`, regexp.QuoteMeta(bootInfo.Uuid)),
			grubCfgContents)
		assert.Regexp(t, fmt.Sprintf(`(?m)^set rootdevice=PARTUUID=%s$`, regexp.QuoteMeta(rootfsInfo.PartUuid)),
			grubCfgContents)

	case baseImageVersionAzl3:
		assert.Regexp(t, fmt.Sprintf(`(?m)[\t ]*search.* --fs-uuid --set=root %s$`, regexp.QuoteMeta(bootInfo.Uuid)),
			grubCfgContents)

		// In theory, UUID should always be used (unless GRUB_DISABLE_UUID is set in the /etc/default/grub file, which
		// it isn't). But on some build hosts, PARTUUID is used instead. Not sure why this is the case. But the OS will
		// still boot either way. So, allow both for now.
		assert.Regexp(t, fmt.Sprintf(`(?m)[\t ]*linux.* root=(UUID=%s|PARTUUID=%s) `, regexp.QuoteMeta(rootfsInfo.Uuid),
			regexp.QuoteMeta(rootfsInfo.PartUuid)),
			grubCfgContents)
	}

	if extraCommandLineArgs != "" {
		assert.Regexp(t, fmt.Sprintf(`(?m)[\t ]*linux.* %s `, regexp.QuoteMeta(extraCommandLineArgs)), grubCfgContents)
	}
}

func getDiskPartitionsMap(devicePath string) (map[int]diskutils.PartitionInfo, error) {
	partitions, err := diskutils.GetDiskPartitions(devicePath)
	if err != nil {
		return nil, err
	}

	partitionsMap := make(map[int]diskutils.PartitionInfo)
	for _, partition := range partitions {
		if partition.Type != "part" {
			continue
		}

		num, err := getPartitionNum(partition.Path)
		if err != nil {
			return nil, err
		}

		partitionsMap[num] = partition
	}

	return partitionsMap, nil
}

func verifyXfsFeature(t *testing.T, partition string, feature string, enabled bool) {
	// For example: " attr=2,"
	featureRegexp, err := regexp.Compile(fmt.Sprintf(`\s+%s=(\w+)(\s|,|$)`, regexp.QuoteMeta(feature)))
	assert.NoError(t, err)

	stdout, _, err := shell.Execute("xfs_info", partition)
	assert.NoError(t, err)

	matches := featureRegexp.FindStringSubmatch(stdout)
	if matches == nil {
		// Feature not found.
		// Xfsprogs is probably too old.
		return
	}

	value := matches[1]
	if enabled {
		assert.Equal(t, "1", value)
	} else {
		assert.Equal(t, "0", value)
	}
}

// verifyBtrfsQuotasDisabled verifies that btrfs quotas are disabled on the filesystem.
func verifyBtrfsQuotasDisabled(t *testing.T, mountDir string) {
	_, _, err := shell.Execute("btrfs", "qgroup", "show", mountDir)
	assert.Error(t, err, "quotas should be disabled (qgroup show should fail)")
}

type btrfsExpectedQuota struct {
	subvolPath      string
	referencedLimit uint64
	exclusiveLimit  uint64
}

// verifyBtrfsQuotas verifies that btrfs quotas are set correctly for the specified subvolumes.
func verifyBtrfsQuotas(t *testing.T, mountDir string, expectedQuotas []btrfsExpectedQuota) {
	qgroups, err := getBtrfsQgroupLimits(mountDir)
	if !assert.NoError(t, err, "get qgroup limits") {
		return
	}

	for _, expected := range expectedQuotas {
		subvolId, err := getBtrfsSubvolumeId(mountDir, expected.subvolPath)
		if !assert.NoErrorf(t, err, "get subvolume ID for %s", expected.subvolPath) {
			continue
		}

		qgroupKey := fmt.Sprintf("0/%d", subvolId)
		qgroup, exists := qgroups[qgroupKey]
		if !assert.Truef(t, exists, "qgroup %s should exist for %s", qgroupKey, expected.subvolPath) {
			continue
		}

		if expected.referencedLimit != qgroup.maxRfer {
			t.Errorf("referenced limit for %s: expected %d, got %d",
				expected.subvolPath, expected.referencedLimit, qgroup.maxRfer)
		}
		if expected.exclusiveLimit != qgroup.maxExcl {
			t.Errorf("exclusive limit for %s: expected %d, got %d",
				expected.subvolPath, expected.exclusiveLimit, qgroup.maxExcl)
		}
	}
}

// verifyBtrfsSubvolumes verifies that the btrfs filesystem has the expected subvolumes.
// If expectedSubvolumes is nil or empty, it verifies that no subvolumes exist.
func verifyBtrfsSubvolumes(t *testing.T, mountDir string, expectedSubvolumes []string) {
	subvolumes, err := listBtrfsSubvolumes(mountDir)
	if !assert.NoError(t, err) {
		return
	}

	if len(expectedSubvolumes) == 0 {
		assert.Empty(t, subvolumes, "btrfs filesystem should have no subvolumes")
	} else {
		assert.ElementsMatch(t, expectedSubvolumes, subvolumes, "btrfs subvolumes should match expected")
	}
}

// verifyBtrfsFstabEntries verifies that the fstab contains the expected btrfs subvolume mount entries.
// expectedOptions maps target paths to their expected mount options.
func verifyBtrfsFstabEntries(t *testing.T, fstabPath string, expectedOptions map[string]string) {
	fstabEntries, err := diskutils.ReadFstabFile(fstabPath)
	if !assert.NoError(t, err, "read /etc/fstab") {
		return
	}

	filteredEntries := filterOutSpecialPartitions(fstabEntries)

	var btrfsTargets []string
	for _, entry := range filteredEntries {
		if entry.FsType == "btrfs" {
			btrfsTargets = append(btrfsTargets, entry.Target)
		}
	}

	expectedTargets := make([]string, 0, len(expectedOptions))
	for target := range expectedOptions {
		expectedTargets = append(expectedTargets, target)
	}
	assert.ElementsMatch(t, expectedTargets, btrfsTargets, "fstab should contain all btrfs subvolume mounts")

	for _, entry := range filteredEntries {
		if entry.FsType == "btrfs" {
			assert.Equalf(t, expectedOptions[entry.Target], entry.Options, "mount options for %s", entry.Target)
		}
	}
}

type btrfsQgroupInfo struct {
	maxRfer uint64
	maxExcl uint64
}

// getBtrfsSubvolumeId returns the subvolume ID for a given subvolume path.
func getBtrfsSubvolumeId(mountDir, subvolPath string) (uint64, error) {
	stdout, _, err := shell.Execute("btrfs", "subvolume", "show", filepath.Join(mountDir, subvolPath))
	if err != nil {
		return 0, fmt.Errorf("failed to get subvolume info for %s: %w", subvolPath, err)
	}

	subvolIdRegex := regexp.MustCompile(`(?m)^\s*Subvolume ID:\s+(\d+)`)
	match := subvolIdRegex.FindStringSubmatch(stdout)
	if match == nil {
		return 0, fmt.Errorf("failed to find Subvolume ID in output for %s", subvolPath)
	}

	var id uint64
	_, err = fmt.Sscanf(match[1], "%d", &id)
	if err != nil {
		return 0, fmt.Errorf("failed to parse subvolume ID: %w", err)
	}

	return id, nil
}

// getBtrfsQgroupLimits returns a map of qgroup ID to quota limits.
func getBtrfsQgroupLimits(mountDir string) (map[string]btrfsQgroupInfo, error) {
	stdout, _, err := shell.Execute("btrfs", "qgroup", "show", "-r", "-e", "--raw", mountDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get qgroup limits: %w", err)
	}

	return parseBtrfsQgroupOutput(stdout)
}

// parseBtrfsQgroupOutput parses the output of `btrfs qgroup show -r -e --raw`.
// Example output:
//
//	qgroupid         rfer         excl     max_rfer     max_excl
//	--------         ----         ----     --------     --------
//	0/5             16384        16384         none         none
//	0/256         4194304      4194304    524288000         none
func parseBtrfsQgroupOutput(output string) (map[string]btrfsQgroupInfo, error) {
	result := make(map[string]btrfsQgroupInfo)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header lines.
		if line == "" || strings.HasPrefix(line, "qgroupid") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		qgroupId := fields[0]
		maxRfer := parseQgroupValue(fields[3])
		maxExcl := parseQgroupValue(fields[4])

		result[qgroupId] = btrfsQgroupInfo{
			maxRfer: maxRfer,
			maxExcl: maxExcl,
		}
	}

	return result, nil
}

// parseQgroupValue parses a qgroup limit value, returning 0 for "none".
func parseQgroupValue(s string) uint64 {
	if s == "none" {
		return 0
	}
	var val uint64
	fmt.Sscanf(s, "%d", &val)
	return val
}
