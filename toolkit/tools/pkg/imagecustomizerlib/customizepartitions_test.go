// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imageconnection"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImagePartitions(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsToEfi(t, "TestCustomizeImagePartitions"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsToEfi(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
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
}

func TestCustomizeImagePartitionsSizeOnly(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImagePartitionsSizeOnly")
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

	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImagePartitionsLegacy(t, "TestCustomizeImagePartitionsLegacy"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImagePartitionsLegacy(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
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
		coreLegacyMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, coreLegacyMountPoints, partitions)
	verifyBootGrubCfg(t, imageConnection, "",
		partitions[coreLegacyMountPoints[0].PartitionNum],
		partitions[coreLegacyMountPoints[0].PartitionNum],
		baseImageInfo)

	// Check the partition types.
	assert.Equal(t, "21686148-6449-6e6f-744e-656564454649", partitions[1].PartitionTypeUuid) // BIOS boot
	assert.Equal(t, "0fc63daf-8483-4772-8e79-3d69d8477de4", partitions[2].PartitionTypeUuid) // linux generic

	// Check the partition sizes.
	assert.Equal(t, uint64(8*diskutils.MiB), partitions[1].SizeInBytes)
	assert.Equal(t, uint64(4086*diskutils.MiB), partitions[2].SizeInBytes)
}

func TestCustomizeImageKernelCommandLine(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageKernelCommandLineHelper(t, "TestCustomizeImageKernelCommandLine"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageKernelCommandLineHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	buildDir := filepath.Join(tmpDir, testName)
	configFile := filepath.Join(testDir, "extracommandline-config.yaml")
	outImageFilePath := filepath.Join(buildDir, "image.qcow2")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
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
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageNewUUIDsHelper(t, "TestCustomizeImageNewUUIDs"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageNewUUIDsHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
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

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
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
	verifyFstabEntries(t, imageConnection, coreEfiMountPoints, newImagePartitions)
	verifyBootloaderConfig(t, imageConnection, "",
		newImagePartitions[coreEfiMountPoints[0].PartitionNum],
		newImagePartitions[coreEfiMountPoints[0].PartitionNum],
		baseImageInfo)
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
