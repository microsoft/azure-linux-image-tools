// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestGrowPartitionSize(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs", "btrfs"} {
		t.Run(fsType, func(t *testing.T) {
			testGrowPartitionSize(t, fsType)
		})
	}
}

func testGrowPartitionSize(t *testing.T, fileSystemType string) {
	if testing.Short() {
		t.Skip("Short mode enabled")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root")
	}

	diskSize := uint64(1 * diskutils.GiB)
	partSize1 := uint64(512 * diskutils.MiB)
	partSize2 := uint64(1022 * diskutils.MiB)
	sectorSize := uint64(512)

	testTmpDir := filepath.Join(tmpDir, "TestGrowPartitionSize_"+fileSystemType)
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
				End:    1 + partSize1/diskutils.MiB,
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
	err = growPartitionSize(loopback.DevicePath(), partPath, partSize2/sectorSize, fileSystemType, buildDir)
	if !assert.NoError(t, err) {
		return
	}

	// Check the filesystem size.
	fileSystemSize, _, err := getPartitionsFilesystemSize(buildDir, partPath, fileSystemType)
	if !assert.NoError(t, err) {
		return
	}

	// ext4 and xfs reports the filesystem size with overhead excluded.
	assert.LessOrEqual(t, uint64(float64(partSize2)*0.9), fileSystemSize)
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
