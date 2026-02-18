// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateImageRaw(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImageRaw")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "create-minimal-os.yaml")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")
	outputImageFormat := "raw"
	noChangeConfigFile := filepath.Join(testDir, "create-minimal-os.yaml")
	vhdFixedImageFilePath := filepath.Join(testTmpDir, "image2.vhd")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", "3.0", false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", "3.0", true)

	err := CreateImageWithConfigFile(
		t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsFile,
		outputImageFilePath, outputImageFormat, "azurelinux", "3.0", "")
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageInfo, err := GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, int64(1*diskutils.GiB), imageInfo.VirtualSize)

	// Customize image to vhd.
	err = CustomizeImageWithConfigFile(
		t.Context(), buildDir, noChangeConfigFile, outputImageFilePath, rpmSources,
		vhdFixedImageFilePath, "vhd", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}
	fileType, err = testutils.GetImageFileType(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd", fileType)

	imageInfo, err = GetImageFileInfo(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vpc", imageInfo.Format)
	assert.Equal(t, int64(1*diskutils.GiB), imageInfo.VirtualSize)
}

func TestCreateImageBtrfs(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImageBtrfs")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "create-minimal-os-btrfs.yaml")
	outputImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outputImageFormat := "raw"

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", "3.0", false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", "3.0", true)

	err := CreateImageWithConfigFile(
		t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsFile,
		outputImageFilePath, outputImageFormat, "azurelinux", "3.0", "")
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageInfo, err := GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, int64(1*diskutils.GiB), imageInfo.VirtualSize)

	// Connect to image and verify btrfs filesystem
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "btrfs",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outputImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify btrfs root filesystem exists
	_, err = os.Stat(filepath.Join(imageConnection.Chroot().RootDir(), "/usr/bin/bash"))
	assert.NoError(t, err, "check for /usr/bin/bash on btrfs root")
}

func TestCreateImageRawNoTar(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImageRawNoTar")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "create-minimal-os.yaml")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", "3.0", false, true)
	rpmSources := []string{downloadedRpmsRepoFile}

	err := CreateImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, rpmSources, "",
		outputImageFilePath, "raw", "azurelinux", "3.0", "")

	assert.ErrorContains(t, err, "tools tar file is required for image creation")
}

func TestCreateImageEmptyConfig(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCreateImageEmptyConfig")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// create an empty config file
	emptyConfigFile := filepath.Join(testDir, "empty-config.yaml")

	err := CreateImageWithConfigFile(t.Context(), buildDir, "", []string{}, "", outputImageFilePath, "raw",
		"azurelinux", "3.0", "")
	assert.ErrorContains(t, err, "failed to unmarshal config file")

	err = CreateImageWithConfigFile(t.Context(), buildDir, emptyConfigFile, []string{}, "", outputImageFilePath, "raw",
		"azurelinux", "3.0", "")
	assert.ErrorContains(t, err, "failed to unmarshal config file")
}

func TestCreateImage_OutputImageFileAsRelativePath(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImage_OutputImageFileAsRelativePathOnCommandLine")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	baseConfigPath := testTmpDir
	configPath := filepath.Join(testDir, "create-minimal-os.yaml")
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configPath, &config)
	assert.NoError(t, err)

	rpmSources := []string{testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", "3.0", false, true)}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", "3.0", true)
	outputImageFileAbsolute := filepath.Join(buildDir, "image1.raw")

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	outputImageFileRelativeToCwd, err := filepath.Rel(cwd, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFileRelativeToConfig, err := filepath.Rel(baseConfigPath, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFile := outputImageFileRelativeToCwd
	outputImageFormat := filepath.Ext(outputImageFile)[1:]

	// Pass the output image file relative to the current working directory through the argument.
	// This will create the file at the absolute path.
	err = createNewImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsFile, "azurelinux", "3.0", "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create
	// the file at the absolute path.
	err = createNewImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsFile, "azurelinux", "3.0", "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)
}
