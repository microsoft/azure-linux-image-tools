// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateImageRaw(t *testing.T) {
	for _, imageInfo := range []testImageInfo{testImageAzl3, testImageAzl4} {
		t.Run(imageInfo.ImageName, func(t *testing.T) {
			testCreateImageRaw(t, imageInfo)
		})
	}
}

func testCreateImageRaw(t *testing.T, imageInfo testImageInfo) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImage_%s", imageInfo.ImageName))
	defer os.RemoveAll(testTmpDir)

	configFileName := ""
	switch imageInfo.Version {
	case "3.0":
		configFileName = "create-minimal-os.yaml"

	case "4.0":
		configFileName = fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)
	}

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, configFileName)
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")
	outputImageFormat := "raw"
	outputImageFilePath2 := filepath.Join(testTmpDir, "image2.raw")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, imageInfo.Distro, imageInfo.Version,
		false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsDir := testutils.GetDownloadedToolsDir(t, testutilsDir, imageInfo.Distro, imageInfo.Version, true)

	err := basicCreateImageWithConfigFile(
		t.Context(), buildDir, configFile, rpmSources, toolsDir,
		outputImageFilePath, outputImageFormat, imageInfo.Distro, imageInfo.Version, imageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	verifyCreateMinimalOs(t, buildDir, outputImageFilePath, imageInfo, false /*btrfs*/)

	// Customize image, to verify the created image is customizable.
	err = CustomizeImageWithConfigFile(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:          buildDir,
		InputImageFile:    outputImageFilePath,
		RpmsSources:       rpmSources,
		OutputImageFile:   outputImageFilePath2,
		OutputImageFormat: "raw",
	})
	if !assert.NoError(t, err) {
		return
	}

	verifyCreateMinimalOs(t, buildDir, outputImageFilePath2, imageInfo, false /*btrfs*/)
}

func TestCreateImageBtrfs(t *testing.T) {
	for _, imageInfo := range []testImageInfo{testImageAzl3, testImageAzl4} {
		t.Run(imageInfo.ImageName, func(t *testing.T) {
			testCreateImageBtrfs(t, imageInfo)
		})
	}
}

func testCreateImageBtrfs(t *testing.T, imageInfo testImageInfo) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImageBtrfs_%s", imageInfo.ImageName))
	defer os.RemoveAll(testTmpDir)

	configFileName := ""
	switch imageInfo.Version {
	case "3.0":
		configFileName = "create-minimal-os-btrfs.yaml"

	case "4.0":
		configFileName = fmt.Sprintf("create-azl4-btrfs-%s.yaml", runtime.GOARCH)
	}

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, configFileName)
	outputImageFilePath := filepath.Join(testTmpDir, "image.raw")
	outputImageFormat := "raw"

	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, imageInfo.Distro, imageInfo.Version,
		false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsDir := testutils.GetDownloadedToolsDir(t, testutilsDir, imageInfo.Distro, imageInfo.Version, true)

	err := basicCreateImageWithConfigFile(
		t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsDir,
		outputImageFilePath, outputImageFormat, imageInfo.Distro, imageInfo.Version, imageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	verifyCreateMinimalOs(t, buildDir, outputImageFilePath, imageInfo, true /*btrfs*/)
}

func TestCreateImageNoTools(t *testing.T) {
	for _, imageInfo := range []testImageInfo{testImageAzl3, testImageAzl4} {
		t.Run(imageInfo.ImageName, func(t *testing.T) {
			testCreateImageNoTools(t, imageInfo)
		})
	}
}

func testCreateImageNoTools(t *testing.T, imageInfo testImageInfo) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImageNoTools_%s", imageInfo.ImageName))
	defer os.RemoveAll(testTmpDir)

	configFileName := ""
	switch imageInfo.Version {
	case "3.0":
		configFileName = "create-minimal-os.yaml"

	case "4.0":
		configFileName = fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)
	}

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, configFileName)
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// Use testDir as a dummy RPM source — validation rejects missing tools dir before
	// any RPM resolution occurs, so the source does not need to be a real repo.
	rpmSources := []string{testDir}

	err := basicCreateImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, rpmSources, "",
		outputImageFilePath, "raw", imageInfo.Distro, imageInfo.Version, imageInfo.PreviewFeatures)

	assert.ErrorContains(t, err, "tools directory is required for image creation")
}

func TestCreateImageEmptyConfig(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCreateImageEmptyConfig")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// create an empty config file
	emptyConfigFile := filepath.Join(testDir, "empty-config.yaml")

	err := basicCreateImageWithConfigFile(t.Context(), buildDir, "", []string{}, "", outputImageFilePath, "raw",
		"azurelinux", "3.0", nil)
	assert.ErrorContains(t, err, "failed to unmarshal config file")

	err = basicCreateImageWithConfigFile(t.Context(), buildDir, emptyConfigFile, []string{}, "", outputImageFilePath, "raw",
		"azurelinux", "3.0", nil)
	assert.ErrorContains(t, err, "failed to unmarshal config file")
}

func TestCreateImage_OutputImageFileAsRelativePath(t *testing.T) {
	for _, imageInfo := range []testImageInfo{testImageAzl3, testImageAzl4} {
		t.Run(imageInfo.ImageName, func(t *testing.T) {
			testCreateImage_OutputImageFileAsRelativePath(t, imageInfo)
		})
	}
}

func testCreateImage_OutputImageFileAsRelativePath(t *testing.T, imageInfo testImageInfo) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCreateImage_OutputImageFileAsRelativePath_%s",
		imageInfo.ImageName))
	defer os.RemoveAll(testTmpDir)

	configFileName := ""
	switch imageInfo.Version {
	case "3.0":
		configFileName = "create-minimal-os.yaml"

	case "4.0":
		configFileName = fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)
	}

	buildDir := filepath.Join(testTmpDir, "build")
	baseConfigPath := testDir
	configPath := filepath.Join(testDir, configFileName)
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configPath, &config)
	assert.NoError(t, err)

	rpmSources := []string{testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, imageInfo.Distro, imageInfo.Version,
		false, true)}
	toolsDir := testutils.GetDownloadedToolsDir(t, testutilsDir, imageInfo.Distro, imageInfo.Version, true)
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
	err = basicCreateImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsDir, imageInfo.Distro, imageInfo.Version, imageInfo.PreviewFeatures)
	assert.NoError(t, err)

	verifyCreateMinimalOs(t, buildDir, outputImageFileAbsolute, imageInfo, false /*btrfs*/)

	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create
	// the file at the absolute path.
	err = basicCreateImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsDir, imageInfo.Distro, imageInfo.Version, imageInfo.PreviewFeatures)
	assert.NoError(t, err)

	verifyCreateMinimalOs(t, buildDir, outputImageFileAbsolute, imageInfo, false /*btrfs*/)
}

func verifyCreateMinimalOs(t *testing.T, buildDir string, outputImageFilePath string, imageInfo testImageInfo,
	btrfs bool,
) {
	distroHandler, err := NewDistroHandler(imageInfo.TargetOs())
	assert.NoError(t, err)

	expectedVirtualSize := int64(3 * diskutils.GiB)

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageFileInfo, err := GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageFileInfo.Format)
	assert.Equal(t, expectedVirtualSize, imageFileInfo.VirtualSize)

	mountPoints := azureLinuxCoreEfiMountPoints
	if btrfs {
		mountPoints = []testutils.MountPoint{
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
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outputImageFilePath, false, /*includeDefaultMounts*/
		mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	packageList, err := distroHandler.GetAllPackagesFromChroot(imageConnection.Chroot())
	if !assert.NoError(t, err, "failed to get package list") {
		return
	}

	expectedPackages := []string{"bash", "rpm", "systemd", "kernel"}
	ensurePackagesInstalled(t, packageList, expectedPackages...)

	ensureFilesExist(t, imageConnection,
		"/usr/bin/bash",
	)

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}
}

func basicCreateImageWithConfigFile(ctx context.Context, buildDir string, configFile string, rpmsSources []string,
	toolsDir string, outputImageFile string, outputImageFormat string, distro string, distroVersion string,
	previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	return CreateImageWithConfigFile(ctx, configFile, ImageCreateOptions{
		BuildDir:          buildDir,
		RpmsSources:       rpmsSources,
		ToolsDir:          toolsDir,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatType(outputImageFormat),
		Distro:            targetos.Distro(distro),
		DistroVersion:     distroVersion,
		PreviewFeatures:   previewFeatures,
	})
}

func basicCreateImage(ctx context.Context, buildDir string, baseConfigPath string, config imagecustomizerapi.Config,
	rpmsSources []string, outputImageFile string, outputImageFormat string, toolsDir string, distro string,
	distroVersion string, previewFeatures []imagecustomizerapi.PreviewFeature,
) error {
	return CreateImage(ctx, baseConfigPath, config, ImageCreateOptions{
		BuildDir:          buildDir,
		RpmsSources:       rpmsSources,
		ToolsDir:          toolsDir,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatType(outputImageFormat),
		Distro:            targetos.Distro(distro),
		DistroVersion:     distroVersion,
		PreviewFeatures:   previewFeatures,
	})
}
