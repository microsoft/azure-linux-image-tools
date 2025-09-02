package imagecreatorlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
	"github.com/stretchr/testify/assert"
)

func TestCreateImageRaw(t *testing.T) {
	checkSkipForCreateImage(t, runCreateImageTests)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImageRaw")
	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "minimal-os.yaml")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")
	outputImageFormat := "raw"
	noChangeConfigFile := filepath.Join(testDir, "minimal-os.yaml")
	vhdFixedImageFilePath := filepath.Join(testTmpDir, "image2.vhd")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "3.0", false, true)
	rpmSources := []string{downloadedRpmsRepoFile}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "3.0", true)

	err := CreateImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, rpmSources, toolsFile, outputImageFilePath, outputImageFormat, "")
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", fileType)

	imageInfo, err := imagecustomizerlib.GetImageFileInfo(outputImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, int64(1*diskutils.GiB), imageInfo.VirtualSize)

	// Customize image to vhd.
	err = imagecustomizerlib.CustomizeImageWithConfigFile(t.Context(), buildDir, noChangeConfigFile, outputImageFilePath, rpmSources, vhdFixedImageFilePath,
		"vhd", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}
	fileType, err = testutils.GetImageFileType(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd", fileType)

	imageInfo, err = imagecustomizerlib.GetImageFileInfo(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vpc", imageInfo.Format)
	assert.Equal(t, int64(1*diskutils.GiB), imageInfo.VirtualSize)
}

func TestCreateImageRawNoTar(t *testing.T) {
	checkSkipForCreateImage(t, runCreateImageTests)

	testTmpDir := filepath.Join(tmpDir, "TestCreateImageRaw")
	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "minimal-os.yaml")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// get RPM sources
	downloadedRpmsRepoFile := testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "3.0", false, true)
	rpmSources := []string{downloadedRpmsRepoFile}

	err := CreateImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, rpmSources, "", outputImageFilePath, "raw", "")

	assert.ErrorContains(t, err, "tools tar file is required for image creation")
}

func TestCreateImageEmptyConfig(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCreateImageRaw")
	buildDir := filepath.Join(testTmpDir, "build")
	outputImageFilePath := filepath.Join(testTmpDir, "image1.raw")

	// create an empty config file
	emptyConfigFile := filepath.Join(testDir, "empty-config.yaml")

	err := CreateImageWithConfigFile(t.Context(), buildDir, "", []string{}, "", outputImageFilePath, "raw", "")
	assert.ErrorContains(t, err, "failed to unmarshal config file")

	err = CreateImageWithConfigFile(t.Context(), buildDir, emptyConfigFile, []string{}, "", outputImageFilePath, "raw", "")
	assert.ErrorContains(t, err, "failed to unmarshal config file")
}

func TestCreateImage_OutputImageFileAsRelativePath(t *testing.T) {
	checkSkipForCreateImage(t, runCreateImageTests)

	buildDir := filepath.Join(tmpDir, "TestCreateImage_OutputImageFileAsRelativePathOnCommandLine")
	baseConfigPath := buildDir
	ConfigPath := filepath.Join(testDir, "minimal-os.yaml")
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(ConfigPath, &config)
	assert.NoError(t, err)

	rpmSources := []string{testutils.GetDownloadedRpmsRepoFile(t, testutilsDir, "3.0", false, true)}
	toolsFile := testutils.GetDownloadedToolsFile(t, testutilsDir, "3.0", true)
	outputImageFileAbsolute := filepath.Join(buildDir, "image1.raw")

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	outputImageFileRelativeToCwd, err := filepath.Rel(cwd, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFileRelativeToConfig, err := filepath.Rel(baseConfigPath, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFile := outputImageFileRelativeToCwd
	outputImageFormat := filepath.Ext(outputImageFile)[1:]

	// Pass the output image file relative to the current working directory through the argument. This will create
	// the file at the absolute path.
	err = createNewImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsFile, "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create the file at the
	// absolute path.
	err = createNewImage(t.Context(), buildDir, baseConfigPath, config, rpmSources, outputImageFile,
		outputImageFormat, toolsFile, "")
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)
}

func TestCreateImageCreatorParameters_OutputImageFileSelection(t *testing.T) {
	checkSkipForCreateImage(t, runCreateImageTests)

	buildDir := filepath.Join(tmpDir, "TestCreateImageCreatorParameters_OutputImageFileSelection")
	outputImageFilePathAsArg := filepath.Join(buildDir, "image-as-arg.raw")
	outputImageFilePathAsConfig := filepath.Join(buildDir, "image-as-config.raw")
	toolsfile := filepath.Join(buildDir, "tools.tar.gz")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{}
	rpmsSources := []string{}
	outputImageFormat := "vhd"
	outputImageFile := ""
	packageSnapshotTime := ""

	// The output image file is not specified in the config or as an argument, so the output image file will be empty.
	ic, err := createImageCreatorParameters(buildDir, configPath, config,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime, toolsfile)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, "")

	// Pass the output image file only in the config.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value in the config.
	ic, err = createImageCreatorParameters(buildDir, configPath, config,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime, toolsfile)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsConfig)
	assert.Equal(t, ic.outputImageBase, "image-as-config")
	assert.Equal(t, ic.outputImageDir, buildDir)
	assert.Equal(t, ic.toolsTar, toolsfile)

	// Pass the output image file only as an argument.
	config.Output.Image.Path = ""
	outputImageFile = outputImageFilePathAsArg

	// The output image file should be set to the value passed as an argument.
	ic, err = createImageCreatorParameters(buildDir, configPath, config,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime, toolsfile)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
	assert.Equal(t, ic.outputImageBase, "image-as-arg")
	assert.Equal(t, ic.outputImageDir, buildDir)

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCreatorParameters(buildDir, configPath, config,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime, toolsfile)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
	assert.Equal(t, ic.outputImageBase, "image-as-arg")
	assert.Equal(t, ic.outputImageDir, buildDir)
}
