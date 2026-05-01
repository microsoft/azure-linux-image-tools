// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateImageRaw(t *testing.T) {
	for _, vi := range []struct {
		name, version, configFile string
		expectedVirtualSize       int64
	}{
		{"azl3", "3.0", "create-minimal-os.yaml", int64(1 * diskutils.GiB)},
		{"azl4", "4.0", fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH), int64(3 * diskutils.GiB)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testCreateImageRaw(t, vi.name, vi.version, vi.configFile, vi.expectedVirtualSize)
		})
	}
}

func testCreateImageRaw(t *testing.T, name string, version string, configFile string, expectedVirtualSize int64) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImageRaw_%s", name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, configFile)
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")
	outputImageFormat := "raw"
	noChangeConfigFile := filepath.Join(testDir, configFile)
	vhdFixedImageFilePath := filepath.Join(testTmpDir, "image2.vhd")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", version, false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", version, true)

	err := CreateImageWithConfigFile(
		t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsFile,
		outputImageFilePath, outputImageFormat, "azurelinux", version, "")
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageInfo, err := GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, expectedVirtualSize, imageInfo.VirtualSize)

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
	assert.Equal(t, expectedVirtualSize, imageInfo.VirtualSize)
}

func TestCreateImageBtrfs(t *testing.T) {
	for _, vi := range []struct {
		name, version, configFile string
		expectedVirtualSize       int64
	}{
		{"azl3", "3.0", "create-minimal-os-btrfs.yaml", int64(1 * diskutils.GiB)},
		{"azl4", "4.0", fmt.Sprintf("create-azl4-btrfs-%s.yaml", runtime.GOARCH), int64(3 * diskutils.GiB)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testCreateImageBtrfs(t, vi.name, vi.version, vi.configFile, vi.expectedVirtualSize)
		})
	}
}

func testCreateImageBtrfs(t *testing.T, name string, version string, configFile string, expectedVirtualSize int64) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImageBtrfs_%s", name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, configFile)
	outputImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outputImageFormat := "raw"

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", version, false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", version, true)

	err := CreateImageWithConfigFile(
		t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsFile,
		outputImageFilePath, outputImageFormat, "azurelinux", version, "")
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageInfo, err := GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, expectedVirtualSize, imageInfo.VirtualSize)

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
	for _, vi := range []struct {
		name, version, configFile string
	}{
		{"azl3", "3.0", "create-minimal-os.yaml"},
		{"azl4", "4.0", fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testCreateImageRawNoTar(t, vi.name, vi.version, vi.configFile)
		})
	}
}

func testCreateImageRawNoTar(t *testing.T, name string, version string, configFile string) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImageRawNoTar_%s", name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, configFile)
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", version, false, true)
	rpmSources := []string{downloadedRpmsRepoFile}

	err := CreateImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, rpmSources, "",
		outputImageFilePath, "raw", "azurelinux", version, "")

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
	for _, vi := range []struct {
		name, version, configFile string
	}{
		{"azl3", "3.0", "create-minimal-os.yaml"},
		{"azl4", "4.0", fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testCreateImage_OutputImageFileAsRelativePath(t, vi.name, vi.version, vi.configFile)
		})
	}
}

func testCreateImage_OutputImageFileAsRelativePath(t *testing.T, name string, version string, configFile string) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImage_OutputImageFileAsRelativePath_%s", name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	baseConfigPath := testDir
	configPath := filepath.Join(testDir, configFile)
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configPath, &config)
	assert.NoError(t, err)

	rpmSources := []string{testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "azurelinux", version, false, true)}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "azurelinux", version, true)
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
		outputImageFormat, toolsFile, "azurelinux", version, "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create
	// the file at the absolute path.
	err = createNewImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsFile, "azurelinux", version, "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)
}
