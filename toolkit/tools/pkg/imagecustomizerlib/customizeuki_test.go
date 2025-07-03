// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestCustomizeImageVerityUsrUki(t *testing.T) {
	baseImageInfo := testBaseImageAzl3CoreEfi
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageUsrVerityUki")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "verity-usr-uki.yaml")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	mountPoints := []MountPoint{
		{
			PartitionNum:   5,
			Path:           "/",
			FileSystemType: "ext4",
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
			PartitionNum:   3,
			Path:           "/usr",
			FileSystemType: "ext4",
			Flags:          unix.MS_RDONLY,
		},
		{
			PartitionNum:   6,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	imageConnection, err := ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	partitions, err := getDiskPartitionsMap(imageConnection.Loopback().DevicePath())
	assert.NoError(t, err, "get disk partitions")

	// Verify that verity is configured correctly.
	espPath := filepath.Join(imageConnection.chroot.RootDir(), "/boot/efi")
	usrDevice := partitionDevPath(imageConnection, 3)
	usrHashDevice := partitionDevPath(imageConnection, 4)
	verifyVerityUki(t, espPath, usrDevice, usrHashDevice, "PARTUUID="+partitions[3].PartUuid,
		"PARTUUID="+partitions[4].PartUuid, "usr", buildDir, "rd.info", "panic-on-corruption")

	expectedFstabEntries := []diskutils.FstabEntry{
		{
			Source:     "PARTUUID=" + partitions[5].PartUuid,
			Target:     "/",
			FsType:     "ext4",
			Options:    "noexec",
			VfsOptions: 0x8,
			FsOptions:  "",
			Freq:       0,
			PassNo:     1,
		},
		{
			Source:     "PARTUUID=" + partitions[2].PartUuid,
			Target:     "/boot",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[1].PartUuid,
			Target:     "/boot/efi",
			FsType:     "vfat",
			Options:    "umask=0077",
			VfsOptions: 0x0,
			FsOptions:  "umask=0077",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "/dev/mapper/usr",
			Target:     "/usr",
			FsType:     "ext4",
			Options:    "ro",
			VfsOptions: 0x1,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
		{
			Source:     "PARTUUID=" + partitions[6].PartUuid,
			Target:     "/var",
			FsType:     "ext4",
			Options:    "defaults",
			VfsOptions: 0x0,
			FsOptions:  "",
			Freq:       0,
			PassNo:     2,
		},
	}
	filteredFstabEntries := getFilteredFstabEntries(t, imageConnection)
	assert.Equal(t, expectedFstabEntries, filteredFstabEntries)
}
