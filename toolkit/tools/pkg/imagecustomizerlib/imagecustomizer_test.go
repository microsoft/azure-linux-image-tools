// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/stretchr/testify/assert"
)

const (
	testImageRootDirName = "testimageroot"
)

var (
	coreEfiMountPoints = []mountPoint{
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
	}

	coreLegacyMountPoints = []mountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "ext4",
		},
	}
)

func TestCustomizeImageEmptyConfig(t *testing.T) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImageEmptyConfig")
	outImageFilePath := filepath.Join(buildDir, "image.vhd")

	// Customize image.
	err = CustomizeImage(buildDir, buildDir, &imagecustomizerapi.Config{}, baseImage, nil, outImageFilePath,
		"vhd",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	// Check output file type.
	checkFileType(t, outImageFilePath, "vhd")
}

func TestCustomizeImageVhd(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageVhd")
	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "partitions-config.yaml")
	noChangeConfigFile := filepath.Join(testDir, "partitions-config.yaml")
	vhdImageFilePath := filepath.Join(testTmpDir, "image1.vhd")
	vhdFixedImageFilePath := filepath.Join(testTmpDir, "image2.vhd")
	vhdxImageFilePath := filepath.Join(testTmpDir, "image3.vhdx")

	// Customize image to vhd.
	err := CustomizeImageWithConfigFile(buildDir, partitionsConfigFile, baseImage, nil, vhdImageFilePath,
		"vhd", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := getImageFileType(vhdImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd", fileType)

	imageInfo, err := getImageFileInfo(vhdImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vpc", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize)

	// Customize image to vhd-fixed.
	err = CustomizeImageWithConfigFile(buildDir, noChangeConfigFile, vhdImageFilePath, nil, vhdFixedImageFilePath,
		"vhd-fixed", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err = getImageFileType(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd-fixed", fileType)

	// qemu-img info detects fixed-length VHDs as raw images.
	// So, subtract VHD footer from disk size.
	imageInfo, err = getImageFileInfo(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize-512)

	// Customize image to vhdx.
	err = CustomizeImageWithConfigFile(buildDir, noChangeConfigFile, vhdFixedImageFilePath, nil, vhdxImageFilePath,
		"vhdx", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err = getImageFileType(vhdxImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhdx", fileType)

	imageInfo, err = getImageFileInfo(vhdxImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhdx", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize)
}

func connectToCoreEfiImage(buildDir string, imageFilePath string) (*ImageConnection, error) {
	return connectToImage(buildDir, imageFilePath, false /*includeDefaultMounts*/, coreEfiMountPoints)
}

type mountPoint struct {
	PartitionNum   int
	Path           string
	FileSystemType string
	Flags          uintptr
}

func connectToImage(buildDir string, imageFilePath string, includeDefaultMounts bool, mounts []mountPoint,
) (*ImageConnection, error) {
	imageConnection := NewImageConnection()
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	rootDir := filepath.Join(buildDir, testImageRootDirName)

	mountPoints := []*safechroot.MountPoint(nil)
	for _, mount := range mounts {
		devPath := partitionDevPath(imageConnection, mount.PartitionNum)

		var mountPoint *safechroot.MountPoint
		if mount.Path == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags,
				"")
		} else {
			mountPoint = safechroot.NewMountPoint(devPath, mount.Path, mount.FileSystemType, mount.Flags, "")
		}

		mountPoints = append(mountPoints, mountPoint)
	}

	err = imageConnection.ConnectChroot(rootDir, false, []string{}, mountPoints, includeDefaultMounts)
	if err != nil {
		imageConnection.Close()
		return nil, err
	}

	return imageConnection, nil
}

func partitionDevPath(imageConnection *ImageConnection, partitionNum int) string {
	devPath := fmt.Sprintf("%sp%d", imageConnection.Loopback().DevicePath(), partitionNum)
	return devPath
}

func TestValidateConfig_CallsValidateInput(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// Test that the input is being validated in validateConfig by
	// triggering an error in validateInput.
	err := validateConfig(testDir, config, "" /*inputImageFile*/, nil, "./out/image.vhdx", "vhdx", true)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "input image file must be specified")
}

func TestValidateInput_AcceptsValidPaths(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// The input image file is not specified in the config, but is
	// specified as an argument, so it should not return an error.
	err := validateConfig(testDir, config, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)

	config.Input.Image.Path = "./base.vhdx"

	// The input image file is specified in both the config and as an
	// argument, so it should not return an error.
	err = validateConfig(testDir, config, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)

	// The input image file is still specified in the config, but not as
	// an argument, so it should still not return an error.
	err = validateConfig(testDir, config, "" /*inputImageFile*/, nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)
}

func TestValidateConfigValidAdditionalFiles(t *testing.T) {
	err := validateConfig(testDir, &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "files/a.txt",
					Destination: "/a.txt",
				},
			},
		},
	}, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)
}

func TestValidateConfigMissingAdditionalFiles(t *testing.T) {
	err := validateConfig(testDir, &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "files/missing_a.txt",
					Destination: "/a.txt",
				},
			},
		},
	}, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.Error(t, err)
}

func TestValidateConfigdditionalFilesIsDir(t *testing.T) {
	err := validateConfig(testDir, &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "files",
					Destination: "/a.txt",
				},
			},
		},
	}, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.Error(t, err)
}

func TestValidateConfigScript(t *testing.T) {
	err := validateScripts(testDir, &imagecustomizerapi.Scripts{
		PostCustomization: []imagecustomizerapi.Script{
			{
				Path: "scripts/postcustomizationscript.sh",
			},
		},
		FinalizeCustomization: []imagecustomizerapi.Script{
			{
				Path: "scripts/finalizecustomizationscript.sh",
			},
		},
	})
	assert.NoError(t, err)
}

func TestValidateConfigScriptNonLocalFile(t *testing.T) {
	err := validateScripts(testDir, &imagecustomizerapi.Scripts{
		FinalizeCustomization: []imagecustomizerapi.Script{
			{
				Path: "../a.sh",
			},
		},
	})
	assert.Error(t, err)
}

func TestValidateConfig_CallsValidateOutput(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// Test that the output is being validated in validateConfig by
	// triggering an error in validateOutput.
	err := validateConfig(testDir, config, "./base.vhdx", nil, "", "vhdx", true)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "output image file must be specified")
}

func TestValidateOutput_AcceptsValidPaths(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// The output image file is not specified in the config, but is
	// specified as an argument, so it should not return an error.
	err := validateConfig(testDir, config, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)

	config.Output.Image.Path = "./out/image.vhdx"

	// The output image file is specified in both the config and as an
	// argument, so it should not return an error.
	err = validateConfig(testDir, config, "./base.vhdx", nil, "./out/image.vhdx", "vhdx", true)
	assert.NoError(t, err)

	// The output image file is still specified in the config, but not as
	// an argument, so it should still not return an error.
	err = validateConfig(testDir, config, "./base.vhdx", nil, "", "vhdx", true)
	assert.NoError(t, err)
}

func TestCustomizeImage_InputImageFileSelection(t *testing.T) {
	inputImagePath := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_InputImageFileSelection")
	outputImagePath := filepath.Join(buildDir, "image.vhd")

	// Pass the input image file only through the config.
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: inputImagePath,
			},
		},
	}
	err := CustomizeImage(buildDir, buildDir, config, "" /*inputImageFile*/, nil, outputImagePath, "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	// Pass the input image file only through the argument.
	config.Input.Image.Path = ""
	err = CustomizeImage(buildDir, buildDir, config, inputImagePath, nil, outputImagePath, "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	// Pass the input image file through both the config and the argument.
	config.Input.Image.Path = filepath.Join(buildDir, "doesnotexist.xxx")
	err = CustomizeImage(buildDir, buildDir, config, inputImagePath, nil, outputImagePath, "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
}

func TestCustomizeImageKernelCommandLineAdd(t *testing.T) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImageKernelCommandLine")
	outImageFilePath := filepath.Join(buildDir, "image.vhd")

	// Customize image.
	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
			},
		},
	}

	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, outImageFilePath, "raw",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	// Mount the output disk image so that its contents can be checked.
	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Read the grub.cfg file.
	grub2ConfigFilePath := filepath.Join(imageConnection.Chroot().RootDir(), installutils.GrubCfgFile)

	grub2ConfigFile, err := os.ReadFile(grub2ConfigFilePath)
	if !assert.NoError(t, err) {
		return
	}

	t.Logf("%s", grub2ConfigFile)

	linuxCommandLineRegex, err := regexp.Compile(`linux .* console=tty0 console=ttyS0 `)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, linuxCommandLineRegex.Match(grub2ConfigFile))
}

func TestCustomizeImage_OutputImageFileSelection(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFileSelection")
	outputImageFilePathAsArg := filepath.Join(buildDir, "image-as-arg.vhd")
	outputImageFilePathAsConfig := filepath.Join(buildDir, "image-as-config.vhd")

	// Pass the output image file only through the config.
	config := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path: outputImageFilePathAsConfig,
			},
		},
	}
	err := CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", "vhd",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFilePathAsConfig)

	// Clean up previous test.
	err = os.Remove(outputImageFilePathAsConfig)
	assert.NoError(t, err)

	// Pass the output image file only through the argument.
	config.Output.Image.Path = ""
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, outputImageFilePathAsArg, "vhd",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.NoFileExists(t, outputImageFilePathAsConfig)
	assert.FileExists(t, outputImageFilePathAsArg)

	// Clean up previous test.
	err = os.Remove(outputImageFilePathAsArg)
	assert.NoError(t, err)

	// Pass the output image file through both the config and the argument.
	config.Output.Image.Path = outputImageFilePathAsConfig
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, outputImageFilePathAsArg, "vhd",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.NoFileExists(t, outputImageFilePathAsConfig)
	assert.FileExists(t, outputImageFilePathAsArg)
}

func TestCustomizeImage_OutputImageFormatSelection(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFormatSelection")
	outputImageFile := filepath.Join(buildDir, "image.dat")
	outputImageFormatAsArg := "vhd"
	outputImageFormatAsConfig := "vhdx"

	// Pass the output image format only through the config.
	config := &imagecustomizerapi.Config{
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   outputImageFile,
				Format: imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig),
			},
		},
	}
	err := CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsConfig)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format only through the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", outputImageFormatAsArg,
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsArg)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format through both the config and the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", outputImageFormatAsArg,
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsArg)
}

func TestCreateImageCustomizerParameters_InputImageFileSelection(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCreateImageCustomizerParameters_InputImageFileSelection")
	inputImageFileAsArg := filepath.Join(buildDir, "image-as-arg.vhdx")
	inputImageFileIsoAsArg := filepath.Join(buildDir, "image-as-arg.iso")
	inputImageFileAsConfig := filepath.Join(buildDir, "image-as-config.vhdx")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileAsArg)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileIsoAsArg)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileAsConfig)
	assert.NoError(t, err)

	// Pass the input image file only in the config.
	inputImageFile := ""
	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: inputImageFileAsConfig,
			},
		},
	}
	useBaseImageRpmRepos := false
	rpmsSources := []string{}
	outputImageFormat := "vhdx"
	outputImageFile := "out/image.vhdx"
	outputPXEArtifactsDir := ""

	// The input image file should be set to the value in the config.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsConfig)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass the input image file only as an argument.
	config.Input.Image.Path = ""
	inputImageFile = inputImageFileAsArg

	// The input image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsArg)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass the input image file in both the config and as an argument.
	config.Input.Image.Path = inputImageFileAsConfig

	// The input image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsArg)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass in an ISO to test that inputIsIso is set correctly.
	inputImageFile = inputImageFileIsoAsArg
	outputImageFormat = "iso"
	outputImageFile = "out/image.iso"
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileIsoAsArg)
	assert.Equal(t, ic.inputImageFormat, "iso")
	assert.True(t, ic.inputIsIso)
}

func TestCreateImageCustomizerParameters_OutputImageFileSelection(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCreateImageCustomizerParameters_OutputImageFileSelection")
	outputImageFilePathAsArg := filepath.Join(buildDir, "image-as-arg.vhd")
	outputImageFilePathAsConfig := filepath.Join(buildDir, "image-as-config.vhd")
	inputImageFile := filepath.Join(buildDir, "image.vhd")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFile)
	assert.NoError(t, err)

	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{}
	useBaseImageRpmRepos := false
	rpmsSources := []string{}
	outputImageFormat := "vhd"
	outputImageFile := ""
	outputPXEArtifactsDir := ""

	// The output image file is not specified in the config or as an
	// argument, so the output image file will be empty.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, "")

	// Pass the output image file only in the config.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value in the config.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsConfig)
	assert.Equal(t, ic.outputImageBase, "image-as-config")
	assert.Equal(t, ic.outputImageDir, buildDir)

	// Pass the output image file only as an argument.
	config.Output.Image.Path = ""
	outputImageFile = outputImageFilePathAsArg

	// The output image file should be set to the value passed as an argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
	assert.Equal(t, ic.outputImageBase, "image-as-arg")
	assert.Equal(t, ic.outputImageDir, buildDir)

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
	assert.Equal(t, ic.outputImageBase, "image-as-arg")
	assert.Equal(t, ic.outputImageDir, buildDir)
}

func TestCreateImageCustomizerParameters_OutputImageFormatSelection(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCreateImageCustomizerParameters_OutputImageFormatSelection")
	inputImageFile := filepath.Join(buildDir, "base.dat")
	outputImageFormatAsArg := "vhd"
	outputImageFormatAsConfig := "vhdx"

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFile)
	assert.NoError(t, err)

	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{}
	useBaseImageRpmRepos := false
	rpmsSources := []string{}
	outputImageFormat := ""
	outputImageFile := filepath.Join(buildDir, "image.vhd")
	outputPXEArtifactsDir := ""

	// The output image format is not specified in the config or as an
	// argument, so the output image format will be empty.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatTypeNone)

	// Pass the output image format only in the config.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)

	// The output image file should be set to the value in the config.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig))

	// Pass the output image format only as an argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	outputImageFormat = outputImageFormatAsArg

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatType(outputImageFormatAsArg))

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, outputPXEArtifactsDir)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatType(outputImageFormatAsArg))
}

func checkFileType(t *testing.T, filePath string, expectedFileType string) {
	fileType, err := getImageFileType(filePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedFileType, fileType)
}

func getImageFileType(filePath string) (string, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	firstBytes := make([]byte, 512)
	firstBytesCount, err := file.Read(firstBytes)
	if err != nil {
		return "", err
	}

	lastBytes := make([]byte, 512)
	lastBytesCount, err := file.ReadAt(lastBytes, max(0, stat.Size()-512))
	if err != nil {
		return "", err
	}

	switch {
	case firstBytesCount >= 8 && bytes.Equal(firstBytes[:8], []byte("conectix")):
		return "vhd", nil

	case firstBytesCount >= 8 && bytes.Equal(firstBytes[:8], []byte("vhdxfile")):
		return "vhdx", nil

	case isZstFile(firstBytes):
		return "zst", nil

	// Check for the MBR signature (which exists even on GPT formatted drives).
	case firstBytesCount >= 512 && bytes.Equal(firstBytes[510:512], []byte{0x55, 0xAA}):
		switch {
		case lastBytesCount >= 512 && bytes.Equal(lastBytes[:8], []byte("conectix")):
			return "vhd-fixed", nil

		default:
			return "raw", nil
		}

	default:
		return "", fmt.Errorf("unknown file type: %s", filePath)
	}
}

func isZstFile(firstBytes []byte) bool {
	if len(firstBytes) < 4 {
		return false
	}

	magicNumber := binary.LittleEndian.Uint32(firstBytes[:4])

	// 0xFD2FB528 is a zst frame.
	// 0x184D2A50-0x184D2A5F are skippable ztd frames.
	return magicNumber == 0xFD2FB528 || (magicNumber >= 0x184D2A50 && magicNumber <= 0x184D2A5F)
}
