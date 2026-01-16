// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageMultiKernel(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageMultiKernel(t, "TestCustomizeImageMultiKernel"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageMultiKernel(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	configFile := ""
	switch baseImageInfo.Version {
	case baseImageVersionAzl2:
		configFile = filepath.Join(testDir, "multikernel-azl2.yaml")

	case baseImageVersionAzl3:
		configFile = filepath.Join(testDir, "multikernel-azl3.yaml")
	}

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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

	linuxCommandRegex := regexp.MustCompile(`linux.* console=tty0 console=ttyS0 `)
	matches := linuxCommandRegex.FindAllString(grubCfgContents, -1)

	switch baseImageInfo.Version {
	case baseImageVersionAzl2:
		// AZL2's default grub.cfg file doesn't support multiple kernels.
		assert.GreaterOrEqual(t, len(matches), 1, "grub.cfg:\n%s", grubCfgContents)

	case baseImageVersionAzl3:
		// There should be multiple matching linux kernels, one for each installed kernel.
		assert.GreaterOrEqual(t, len(matches), 2, "grub.cfg:\n%s", grubCfgContents)
	}
}

func TestFindRootMountPoint_DirectFilesystem_Pass(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			DeviceId: "esp",
			Type:     imagecustomizerapi.FileSystemTypeFat32,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:   "/boot/efi",
				IdType: imagecustomizerapi.MountIdentifierTypePartUuid,
			},
		},
		{
			DeviceId: "rootfs",
			Type:     imagecustomizerapi.FileSystemTypeXfs,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:   "/",
				IdType: imagecustomizerapi.MountIdentifierTypeUuid,
			},
		},
	}

	result := findRootMountPoint(fileSystems)
	assert.NotNil(t, result)
	assert.Equal(t, "/", result.Path)
	assert.Equal(t, imagecustomizerapi.MountIdentifierTypeUuid, result.IdType)
}

func TestFindRootMountPoint_BtrfsSubvolume_Pass(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			DeviceId: "esp",
			Type:     imagecustomizerapi.FileSystemTypeFat32,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:   "/boot/efi",
				IdType: imagecustomizerapi.MountIdentifierTypePartUuid,
			},
		},
		{
			DeviceId: "btrfspart",
			Type:     imagecustomizerapi.FileSystemTypeBtrfs,
			Btrfs: &imagecustomizerapi.BtrfsConfig{
				Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
					{
						Path: "root",
						MountPoint: &imagecustomizerapi.MountPoint{
							Path:   "/",
							IdType: imagecustomizerapi.MountIdentifierTypePartUuid,
						},
					},
					{
						Path: "home",
						MountPoint: &imagecustomizerapi.MountPoint{
							Path:   "/home",
							IdType: imagecustomizerapi.MountIdentifierTypePartUuid,
						},
					},
				},
			},
		},
	}

	result := findRootMountPoint(fileSystems)
	assert.NotNil(t, result)
	assert.Equal(t, "/", result.Path)
	assert.Equal(t, imagecustomizerapi.MountIdentifierTypePartUuid, result.IdType)
}

func TestFindRootMountPoint_NoRoot_Fail(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			DeviceId: "esp",
			Type:     imagecustomizerapi.FileSystemTypeFat32,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:   "/boot/efi",
				IdType: imagecustomizerapi.MountIdentifierTypePartUuid,
			},
		},
	}

	result := findRootMountPoint(fileSystems)
	assert.Nil(t, result)
}
