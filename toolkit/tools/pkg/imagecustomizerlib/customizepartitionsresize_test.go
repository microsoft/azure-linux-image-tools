// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests resizing a filesystem.
func TestGrowPartitionSize(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs", "btrfs"} {
		for _, noFill := range []bool{false, true} {
			name := fsType
			if noFill {
				name += "_nofill"
			}

			t.Run(name, func(t *testing.T) {
				testGrowPartitionSize(t, fsType, noFill)
			})
		}
	}
}

func testGrowPartitionSize(t *testing.T, fileSystemType string, noFill bool) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}

	diskSize := uint64(2 * diskutils.GiB)
	partSizeStart := uint64(512 * diskutils.MiB)
	partSizeNew := uint64(2046 * diskutils.MiB)
	fileSystemSizeNoFill := uint64(1024 * diskutils.MiB)
	sectorSize := uint64(512)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestGrowPartitionSize_%s_%v", fileSystemType, noFill))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	err := os.MkdirAll(buildDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	diskFilePath := filepath.Join(testTmpDir, "disk.raw")

	// Create an oversized disk with a single partition.
	err = diskutils.CreateSparseDisk(diskFilePath, diskSize/diskutils.MiB, 0o644)
	if !assert.NoError(t, err) {
		return
	}

	loopback, err := safeloopback.NewLoopback(diskFilePath)
	if !assert.NoError(t, err) {
		return
	}

	diskConfig := configuration.Disk{
		PartitionTableType: configuration.PartitionTableTypeGpt,
		MaxSize:            diskSize,
		Partitions: []configuration.Partition{
			{
				ID:     "a",
				FsType: fileSystemType,
				Start:  1, // MiB
				End:    1 + partSizeStart/diskutils.MiB,
			},
		},
	}

	partDevPathMap, _, _, err := diskutils.CreatePartitions(targetos.TargetOsAzureLinux3, loopback.DevicePath(),
		diskConfig, configuration.RootEncryption{})
	if !assert.NoError(t, err) {
		return
	}

	partPath := partDevPathMap["a"]

	// Resize the partition.
	newFileSystemSize := uint64(0) // Same as partition size
	if noFill {
		newFileSystemSize = fileSystemSizeNoFill
	}

	err = growPartitionSize(loopback.DevicePath(), partPath, partSizeNew/sectorSize, newFileSystemSize,
		fileSystemType, buildDir)
	if !assert.NoError(t, err) {
		return
	}

	// Check the filesystem size.
	fileSystemSize, _, err := getPartitionsFilesystemSize(buildDir, partPath, fileSystemType)
	if !assert.NoError(t, err) {
		return
	}

	// ext4 and xfs reports the filesystem size with overhead excluded.
	assert.LessOrEqual(t, uint64(float64(newFileSystemSize)*0.9), fileSystemSize)
}

func TestCustomizeImageGrowDisk(t *testing.T) {
	for _, baseImageInfo := range slices.Concat(baseImageAzureLinux3Plus, baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGrow(t, baseImageInfo, "Disk")
		})
	}
}

func TestCustomizeImageGrowLastFreeSpace(t *testing.T) {
	for _, baseImageInfo := range slices.Concat(baseImageAzureLinux3Plus, baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGrow(t, baseImageInfo, "LastFreeSpace")
		})
	}
}

func TestCustomizeImageGrowLastSize(t *testing.T) {
	for _, baseImageInfo := range slices.Concat(baseImageAzureLinux3Plus, baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGrow(t, baseImageInfo, "LastSize")
		})
	}
}

func testCustomizeImageGrow(t *testing.T, baseImageInfo testBaseImageInfo, testType string) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageGrow"+testType+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	err := os.MkdirAll(buildDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	configFile := ""
	switch testType {
	case "Disk":
		configFile = filepath.Join(testDir, "partitions-grow-disk.yaml")

	case "LastFreeSpace":
		configFile = filepath.Join(testDir, "partitions-grow-last-freespace.yaml")

	case "LastSize":
		configFile = filepath.Join(testDir, "partitions-grow-last-size.yaml")
	}

	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	_, err = convertImageToRaw(baseImage, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}

	origStat, err := os.Stat(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, /*includeDefaultMounts*/
		baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	origPartitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	if !assert.NoError(t, err) {
		return
	}

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	// Customize image.
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, outImageFilePath,
		"raw", baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	newStat, err := os.Stat(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err = testutils.ConnectToImage(buildDir, outImageFilePath, false, /*includeDefaultMounts*/
		baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	newPartitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	if !assert.NoError(t, err) {
		return
	}

	// Ensure disk size has grown.
	assert.Less(t, origStat.Size(), newStat.Size())

	switch testType {
	case "Disk":
		assert.Equal(t, int64(50*diskutils.GiB), newStat.Size())

	case "LastSize", "LastFreeSpace":
		assert.LessOrEqual(t, int64(50*diskutils.GiB), newStat.Size())
	}

	// Check partition sizes.
	for partNum := range origPartitions {
		if partNum == baseImageInfo.LastPartNum {
			// Ensure the last partitions has grown.
			assert.Less(t, origPartitions[partNum].SizeInBytes, newPartitions[partNum].SizeInBytes)

			switch testType {
			case "LastFreeSpace":
				assert.LessOrEqual(t, uint64(50*diskutils.GiB), newPartitions[partNum].SizeInBytes)

			case "LastSize":
				assert.Equal(t, uint64(50*diskutils.GiB), newPartitions[partNum].SizeInBytes)
			}

		} else {
			// Ensure other partitions remain untouched.
			assert.Equal(t, origPartitions[partNum].SizeInBytes, newPartitions[partNum].SizeInBytes)
		}
	}
}

func TestCustomizeImageResizeLegacyBoot(t *testing.T) {
	// Skip this test on arm64 because the legacy bootloader is not supported.
	if runtime.GOARCH == "arm64" {
		t.Skip("Skipping legacy test for arm64")
	}

	for _, baseImageInfo := range slices.Concat(baseImageAzureLinux3Plus) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageResizeLegacyBoot(t, baseImageInfo)
		})
	}
}

func testCustomizeImageResizeLegacyBoot(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageResizeLegacyBoot"+baseImageInfo.ImageName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "legacyboot-partitions-azl3.yaml")
	if baseImageInfo.Version == baseImageVersionAzl4 {
		configFile = filepath.Join(testDir, "legacyboot-partitions-azl4.yaml")
	}
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outImageFilePath2 := filepath.Join(testTmpDir, "image2.raw")

	// Customize image.
	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath,
		"raw", baseImageInfo.PreviewFeatures)
	require.NoError(t, err)

	verifyLegacyBootPartitionsImage(t, outImageFilePath, baseImageInfo, buildDir, false /*resized*/)

	// Try to move the BIOS boot partition.
	configFile = filepath.Join(testDir, "legacyboot-partitions-move-biosboot.yaml")
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, outImageFilePath2,
		"raw", baseImageInfo.PreviewFeatures)
	assert.ErrorIs(t, err, ErrDiskResizeCannotMoveBiosBootPartition)

	// Resize a partition that is in the middle of the disk.
	configFile = filepath.Join(testDir, "legacyboot-partitions-resize-home.yaml")
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, outImageFilePath2,
		"raw", baseImageInfo.PreviewFeatures)
	require.NoError(t, err)

	verifyLegacyBootPartitionsImage(t, outImageFilePath2, baseImageInfo, buildDir, true /*resized*/)
}

func verifyLegacyBootPartitionsImage(t *testing.T, outImageFilePath string, baseImageInfo testBaseImageInfo,
	buildDir string, resized bool,
) {
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   1,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   3,
			Path:           "/home",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   4,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Check that the fstab entries are correct.
	verifyFstabEntries(t, imageConnection, mountPoints, partitions, verifyFstabEntryPartuuidSource)
	verifyBootCfg(t, imageConnection, "",
		partitions[mountPoints[0].PartitionNum],
		partitions[mountPoints[0].PartitionNum],
		baseImageInfo)

	// Check the partition types.
	assert.Equal(t, "21686148-6449-6e6f-744e-656564454649", partitions[2].PartitionTypeUuid) // BIOS boot

	// Check the partition sizes.
	assert.Equal(t, uint64(1*diskutils.GiB), partitions[1].SizeInBytes)
	assert.Equal(t, uint64(1*diskutils.MiB), partitions[2].SizeInBytes)
	if resized {
		assert.Equal(t, uint64(1*diskutils.GiB), partitions[3].SizeInBytes)
	} else {
		assert.Equal(t, uint64(500*diskutils.MiB), partitions[3].SizeInBytes)
	}
	assert.Equal(t, uint64(1*diskutils.GiB), partitions[4].SizeInBytes)
}
