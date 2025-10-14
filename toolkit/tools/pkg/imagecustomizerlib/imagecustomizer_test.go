// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

var (
	coreEfiMountPoints = []testutils.MountPoint{
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

	coreLegacyMountPoints = []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "ext4",
		},
	}
)

func TestCustomizeImageEmptyConfig(t *testing.T) {
	var err error

	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageEmptyConfig")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.vhd")

	// Customize image.
	err = CustomizeImage(t.Context(), buildDir, buildDir, &imagecustomizerapi.Config{}, baseImage, nil, outImageFilePath,
		"vhd",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Check output file type.
	checkFileType(t, outImageFilePath, "vhd")
}

func TestCustomizeImageVhd(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageVhd")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	partitionsConfigFile := filepath.Join(testDir, "partitions-config.yaml")
	noChangeConfigFile := filepath.Join(testDir, "partitions-config.yaml")
	vhdImageFilePath := filepath.Join(testTmpDir, "image1.vhd")
	vhdFixedImageFilePath := filepath.Join(testTmpDir, "image2.vhd")
	vhdxImageFilePath := filepath.Join(testTmpDir, "image3.vhdx")

	// Customize image to vhd.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, partitionsConfigFile, baseImage, nil, vhdImageFilePath,
		"vhd", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err := testutils.GetImageFileType(vhdImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd", fileType)

	imageInfo, err := GetImageFileInfo(vhdImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vpc", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize)

	// Customize image to vhd-fixed.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, noChangeConfigFile, vhdImageFilePath, nil, vhdFixedImageFilePath,
		"vhd-fixed", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err = testutils.GetImageFileType(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhd-fixed", fileType)

	// qemu-img info detects fixed-length VHDs as raw images.
	// So, subtract VHD footer from disk size.
	imageInfo, err = GetImageFileInfo(vhdFixedImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "raw", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize-512)

	// Customize image to vhdx.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, noChangeConfigFile, vhdFixedImageFilePath, nil, vhdxImageFilePath,
		"vhdx", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	fileType, err = testutils.GetImageFileType(vhdxImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhdx", fileType)

	imageInfo, err = GetImageFileInfo(vhdxImageFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "vhdx", imageInfo.Format)
	assert.Equal(t, int64(4*diskutils.GiB), imageInfo.VirtualSize)
}

func connectToCoreEfiImage(buildDir string, imageFilePath string) (*imageconnection.ImageConnection, error) {
	return testutils.ConnectToImage(buildDir, imageFilePath, false /*includeDefaultMounts*/, coreEfiMountPoints)
}

func TestValidateConfig_CallsValidateInput(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// Test that the input is being validated in validateConfig by
	// triggering an error in validateInput.
	_, err := ValidateConfig(t.Context(), testDir, config, false,
		ImageCustomizerOptions{
			OutputImageFile:   "./out/image.vhdx",
			OutputImageFormat: "vhdx",
		})
	assert.Error(t, err)
	assert.ErrorContains(t, err, "input image file must be specified")
}

func TestValidateConfig_CallsValidateInput_NewImage(t *testing.T) {
	config := &imagecustomizerapi.Config{}

	// Test that the input is being validated in validateConfig by
	// triggering an error in validateInput.
	_, err := ValidateConfig(t.Context(), testDir, config, true,
		ImageCustomizerOptions{
			OutputImageFile:   "./out/image.raw",
			OutputImageFormat: "raw",
		})
	assert.NoError(t, err)
}

func TestValidateInput_AcceptsValidPaths(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	baseConfigPath := testDir
	config := &imagecustomizerapi.Config{}

	inputImageFileFake := filepath.Join(testDir, "testimages", "doesnotexist.xxx")
	inputImageFileReal := filepath.Join(testDir, "testimages", "empty.vhdx")
	inputImageFileRealRelativeCwd, err := filepath.Rel(cwd, inputImageFileReal)
	assert.NoError(t, err)
	inputImageFileRealRelativeConfig, err := filepath.Rel(baseConfigPath, inputImageFileReal)
	assert.NoError(t, err)

	outputImageFile := "out/image.vhdx"
	defer os.Remove(outputImageFile)

	options := ImageCustomizerOptions{
		InputImageFile:    inputImageFileReal,
		OutputImageFile:   outputImageFile,
		OutputImageFormat: imagecustomizerapi.ImageFormatType(filepath.Ext(outputImageFile)[1:]),
	}

	// The input image file can be specified as an argument without being specified in the config.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.InputImageFile = inputImageFileRealRelativeCwd

	// The input image file specified as an argument can be relative to the current working directory.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.InputImageFile = inputImageFileFake

	// The input image file, specified as an argument, must be a file.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")

	options.InputImageFile = ""
	config.Input.Image.Path = inputImageFileReal

	// The input image file can be specified in the config without being specified as an argument.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileRealRelativeConfig

	// The input image file specified in the config can be relative to the bash config path.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileFake

	// The input image file, specified in the config, must be a file.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")

	options.InputImageFile = inputImageFileReal
	config.Input.Image.Path = inputImageFileReal

	// The input image file can be specified both as an argument and in the config.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileFake

	// The input image file can even be invalid in the config if it is specified as an argument.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)
}

func TestValidateConfigValidAdditionalFiles(t *testing.T) {
	_, err := ValidateConfig(t.Context(), testDir,
		&imagecustomizerapi.Config{
			OS: &imagecustomizerapi.OS{
				AdditionalFiles: imagecustomizerapi.AdditionalFileList{
					{
						Source:      "files/a.txt",
						Destination: "/a.txt",
					},
				},
			},
			Input: imagecustomizerapi.Input{
				Image: imagecustomizerapi.InputImage{
					Path: "testimages/empty.vhdx",
				},
			},
		},
		false,
		ImageCustomizerOptions{
			OutputImageFile:   "./out/image.vhdx",
			OutputImageFormat: "vhdx",
		})
	assert.NoError(t, err)
}

func TestValidateConfigMissingAdditionalFiles(t *testing.T) {
	_, err := ValidateConfig(t.Context(), testDir,
		&imagecustomizerapi.Config{
			OS: &imagecustomizerapi.OS{
				AdditionalFiles: imagecustomizerapi.AdditionalFileList{
					{
						Source:      "files/missing_a.txt",
						Destination: "/a.txt",
					},
				},
			},
			Input: imagecustomizerapi.Input{
				Image: imagecustomizerapi.InputImage{
					Path: "testimages/empty.vhdx",
				},
			},
		}, false,
		ImageCustomizerOptions{
			OutputImageFile:   "./out/image.vhdx",
			OutputImageFormat: "vhdx",
		})
	assert.Error(t, err)
}

func TestValidateConfigdditionalFilesIsDir(t *testing.T) {
	_, err := ValidateConfig(t.Context(), testDir,
		&imagecustomizerapi.Config{
			OS: &imagecustomizerapi.OS{
				AdditionalFiles: imagecustomizerapi.AdditionalFileList{
					{
						Source:      "files",
						Destination: "/a.txt",
					},
				},
			},
			Input: imagecustomizerapi.Input{
				Image: imagecustomizerapi.InputImage{
					Path: "testimages/empty.vhdx",
				},
			},
		}, false,
		ImageCustomizerOptions{
			OutputImageFile:   "./out/image.vhdx",
			OutputImageFormat: "vhdx",
		})
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
	baseConfigPath := testDir
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "testimages/empty.vhdx",
			},
		},
	}
	options := ImageCustomizerOptions{
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhd,
	}

	// Test that the output is being validated in validateConfig by triggering an error in validateOutput.
	_, err := ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "output image file must be specified")
}

func TestValidateOutput_AcceptsValidPaths(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	testTempDir := filepath.Join(tmpDir, "TestValidateOutput_AcceptsValidPaths")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	err = os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	baseConfigPath := testDir
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "testimages/empty.vhdx",
			},
		},
	}

	options := ImageCustomizerOptions{}

	outputImageDir := filepath.Join(testTempDir, "out")
	err = os.MkdirAll(outputImageDir, os.ModePerm)
	assert.NoError(t, err)
	outputImageDirRelativeCwd, err := filepath.Rel(cwd, outputImageDir)
	assert.NoError(t, err)
	outputImageDirRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageDir)
	assert.NoError(t, err)

	outputImageFileNew := filepath.Join(outputImageDir, "new.vhdx")
	outputImageFileNewRelativeCwd, err := filepath.Rel(cwd, outputImageFileNew)
	assert.NoError(t, err)
	outputImageFileNewRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageFileNew)
	assert.NoError(t, err)

	outputImageFileExists := filepath.Join(outputImageDir, "exists.vhdx")
	err = file.Write("", outputImageFileExists)
	assert.NoError(t, err)
	outputImageFileExistsRelativeCwd, err := filepath.Rel(cwd, outputImageFileExists)
	assert.NoError(t, err)
	outputImageFileExistsRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageFileExists)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileNew
	options.OutputImageFormat = imagecustomizerapi.ImageFormatType(filepath.Ext(options.OutputImageFile)[1:])

	// The output image file can be sepcified as an argument without being in specified the config.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileNewRelativeCwd

	// The output image file can be specified as an argument relative to the current working directory.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageDir

	// The output image file, specified as an argument, must not be a directory.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	options.OutputImageFile = outputImageDirRelativeCwd

	// The above is also true for relative paths.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	options.OutputImageFile = outputImageFileExists

	// The output image file, specified as an argument, may be a file that already exists.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileExistsRelativeCwd

	// The above is also true for relative paths.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.OutputImageFile = ""
	config.Output.Image.Path = outputImageFileNew

	// The output image file cab be specified in the config without being specified as an argument.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileNewRelativeConfig

	// The output image file can be specified in the config relative to the base config path.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file, specified in the config, must not be a directory.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageDirRelativeConfig

	// The above is also true for relative paths.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageFileExists

	// The output image file, specified in the config, may be a file that already exists.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileExistsRelativeConfig

	// The above is also true for relative paths.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileNew
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified both as an argument and in the config.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file can even be invalid in the config if it is specified as an argument.
	_, err = ValidateConfig(t.Context(), baseConfigPath, config, false, options)
	assert.NoError(t, err)
}

func TestCustomizeImage_InputImageFileSelection(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestCustomizeImage_InputImageFileSelection")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	baseConfigPath := testTempDir
	config := &imagecustomizerapi.Config{}

	inputImageFileFake := filepath.Join(testTempDir, "doesnotexist.xxx")
	inputImageFileReal, _ := checkSkipForCustomizeDefaultImage(t)

	inputImageFile := inputImageFileReal
	rpmSources := []string{}
	outputImageFile := filepath.Join(testTempDir, "image.vhd")
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the input image file only through the argument.
	err := CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	inputImageFileRealAbs, err := filepath.Abs(inputImageFileReal)
	assert.NoError(t, err)
	config.Input.Image.Path = inputImageFileRealAbs
	inputImageFile = ""

	// Pass the input image file only through the config.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	inputImageFile = inputImageFileReal
	config.Input.Image.Path = inputImageFileFake

	// Pass the input image file through both the config and the argument. The config's Path is ignored, so even though
	// it doesn't exist, there will be no error.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)
}

func TestCustomizeImage_InputImageFileAsRelativePath(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestCustomizeImage_InputImageFileAsRelativePathOnCommandLine")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	baseConfigPath := testTempDir
	config := &imagecustomizerapi.Config{}

	inputImageFileAbsoluteFake := filepath.Join(testTempDir, "doesnotexist.xxx")
	inputImageFileReal, _ := checkSkipForCustomizeDefaultImage(t)

	inputImageFileAbsoluteReal, err := filepath.Abs(inputImageFileReal)
	assert.NoError(t, err)

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	inputImageFileRelativeToCwdReal, err := filepath.Rel(cwd, inputImageFileAbsoluteReal)
	assert.NoError(t, err)
	inputImageFileRelativeToCwdFake, err := filepath.Rel(cwd, inputImageFileAbsoluteFake)
	assert.NoError(t, err)

	inputImageFileRelativeToConfigReal, err := filepath.Rel(baseConfigPath, inputImageFileAbsoluteReal)
	assert.NoError(t, err)
	inputImageFileRelativeToConfigFake, err := filepath.Rel(baseConfigPath, inputImageFileAbsoluteFake)
	assert.NoError(t, err)

	inputImageFile := inputImageFileRelativeToCwdReal
	rpmSources := []string{}
	outputImageFile := filepath.Join(testTempDir, "image.vhd")
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the input image file relative to the current working directory through the argument. This works because
	// paths on the command-line are expected to be relative to the current working directory.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	inputImageFile = inputImageFileRelativeToCwdFake

	// The same as above but for the fake path. This fails because the file does not exist.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")
	assert.NoFileExists(t, outputImageFile)

	config.Input.Image.Path = inputImageFileRelativeToConfigReal
	inputImageFile = ""

	// Pass the input image file relative to the config file through the config. This works because paths in the config
	// as expected to be relative to the config file.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileRelativeToConfigFake

	// The same as above but for the fake path. This fails because the file does not exist.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")
}

func TestCustomizeImageKernelCommandLineAdd(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageKernelCommandLineAddHelper(t, "TestCustomizeImageKernelCommandLineAdd"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageKernelCommandLineAddHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	var err error

	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	buildDir := filepath.Join(tmpDir, testName)
	outImageFilePath := filepath.Join(buildDir, "image.vhd")

	// Customize image.
	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
			},
		},
	}

	err = CustomizeImage(t.Context(), buildDir, buildDir, config, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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

	assert.Regexp(t, `linux\s+.*\s+console=tty0 console=ttyS0\s+`, grub2ConfigFile)
}

func TestCustomizeImage_OutputImageFileSelection(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFileSelection")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	baseConfigPath := testTmpDir
	config := &imagecustomizerapi.Config{}
	inputImageFile, _ := checkSkipForCustomizeDefaultImage(t)
	rpmSources := []string{}

	outputImageFileAsArgument := filepath.Join(testTmpDir, "image-as-arg.vhd")
	outputImageFilePathAsConfig := filepath.Join(testTmpDir, "image-as-config.vhd")

	outputImageFile := outputImageFileAsArgument
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the output image file only through the argument.
	err := CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAsArgument)
	err = os.Remove(outputImageFileAsArgument)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFilePathAsConfig
	outputImageFile = ""

	// Pass the output image file only through the config.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, "",
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFilePathAsConfig)
	err = os.Remove(outputImageFilePathAsConfig)
	assert.NoError(t, err)

	config.Output.Image.Path = buildDir
	outputImageFile = outputImageFileAsArgument

	// Pass the output image file through both the config and the argument. The config's Path is ignored, so even though
	// it is a directory, there will be no error.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAsArgument)
	assert.NoFileExists(t, outputImageFilePathAsConfig)
	err = os.Remove(outputImageFileAsArgument)
	assert.NoError(t, err)
}

func TestCustomizeImage_OutputImageFileAsRelativePath(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFileAsRelativePath")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	baseConfigPath := testTmpDir
	config := &imagecustomizerapi.Config{}
	inputImageFile, _ := checkSkipForCustomizeDefaultImage(t)
	rpmSources := []string{}

	outputImageFileAbsolute := filepath.Join(testTmpDir, "image.vhdx")

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	outputImageFileRelativeToCwd, err := filepath.Rel(cwd, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFileRelativeToConfig, err := filepath.Rel(baseConfigPath, outputImageFileAbsolute)
	assert.NoError(t, err)

	outputImageFile := outputImageFileRelativeToCwd
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the output image file relative to the current working directory through the argument. This will create
	// the file at the absolute path.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create the file at the
	// absolute path.
	err = CustomizeImage(t.Context(), buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)
}

func TestCustomizeImage_OutputImageFormatSelection(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFormatSelection")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outputImageFile := filepath.Join(testTmpDir, "image.raw")
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
	err := CustomizeImage(t.Context(), buildDir, testTmpDir, config, baseImage, nil, "", "",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsConfig)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format only through the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	err = CustomizeImage(t.Context(), buildDir, testTmpDir, config, baseImage, nil, "", outputImageFormatAsArg,
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsArg)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format through both the config and the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)
	err = CustomizeImage(t.Context(), buildDir, testTmpDir, config, baseImage, nil, "", outputImageFormatAsArg,
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsArg)
}

func TestValidateConfig_InputImageFileSelection(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestValidateConfig_InputImageFileSelection")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	inputImageFileAsArg := filepath.Join(testTmpDir, "image-as-arg.vhdx")
	inputImageFileIsoAsArg := filepath.Join(testTmpDir, "image-as-arg.iso")
	inputImageFileAsConfig := filepath.Join(testTmpDir, "image-as-config.vhdx")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileAsArg)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileIsoAsArg)
	assert.NoError(t, err)

	err = file.Write("", inputImageFileAsConfig)
	assert.NoError(t, err)

	// Pass the input image file only in the config.
	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: inputImageFileAsConfig,
			},
		},
	}
	options := ImageCustomizerOptions{
		BuildDir:          buildDir,
		OutputImageFormat: "vhdx",
		OutputImageFile:   "out/image.vhdx",
	}

	// The input image file should be set to the value in the config.
	rc, err := ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.InputImageFile, inputImageFileAsConfig)
	assert.Equal(t, rc.InputFileExt(), "vhdx")
	assert.False(t, rc.InputIsIso())

	// Pass the input image file only as an argument.
	config.Input.Image.Path = ""
	options.InputImageFile = inputImageFileAsArg

	// The input image file should be set to the value passed as an argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.InputImageFile, inputImageFileAsArg)
	assert.Equal(t, rc.InputFileExt(), "vhdx")
	assert.False(t, rc.InputIsIso())

	// Pass the input image file in both the config and as an argument.
	config.Input.Image.Path = inputImageFileAsConfig

	// The input image file should be set to the value passed as an argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.InputImageFile, inputImageFileAsArg)
	assert.Equal(t, rc.InputFileExt(), "vhdx")
	assert.False(t, rc.InputIsIso())

	// Pass in an ISO to test that inputIsIso is set correctly.
	options.InputImageFile = inputImageFileIsoAsArg
	options.OutputImageFormat = "iso"
	options.OutputImageFile = "out/image.iso"
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.InputImageFile, inputImageFileIsoAsArg)
	assert.Equal(t, rc.InputFileExt(), "iso")
	assert.True(t, rc.InputIsIso())
}

func TestValidateConfig_OutputImageFileSelection(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestValidateConfig_OutputImageFileSelection")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outputImageFilePathAsArg := filepath.Join(testTmpDir, "image-as-arg.vhd")
	outputImageFilePathAsConfig := filepath.Join(testTmpDir, "image-as-config.vhd")
	inputImageFile := filepath.Join(testTmpDir, "image.vhd")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFile)
	assert.NoError(t, err)

	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{}

	options := ImageCustomizerOptions{
		BuildDir:          buildDir,
		OutputImageFormat: "vhd",
		InputImageFile:    inputImageFile,
	}

	// The output image file is not specified in the config or as an argument, so the output image file will be empty.
	rc, err := ValidateConfig(t.Context(), configPath, config, false, options)
	assert.ErrorContains(t, err, "output image file must be specified")

	// Pass the output image file only in the config.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value in the config.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFile, outputImageFilePathAsConfig)

	// Pass the output image file only as an argument.
	config.Output.Image.Path = ""
	options.OutputImageFile = outputImageFilePathAsArg

	// The output image file should be set to the value passed as an argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFile, outputImageFilePathAsArg)

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value passed as an
	// argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFile, outputImageFilePathAsArg)
}

func TestValidateConfig_OutputImageFormatSelection(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestValidateConfig_OutputImageFormatSelection")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	inputImageFile := filepath.Join(testTmpDir, "base.dat")
	outputImageFormatAsArg := imagecustomizerapi.ImageFormatType("vhd")
	outputImageFormatAsConfig := imagecustomizerapi.ImageFormatType("vhdx")

	err := os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	err = file.Write("", inputImageFile)
	assert.NoError(t, err)

	configPath := "config.yaml"
	config := &imagecustomizerapi.Config{}

	options := ImageCustomizerOptions{
		BuildDir:        buildDir,
		OutputImageFile: filepath.Join(testTmpDir, "image.vhd"),
		InputImageFile:  inputImageFile,
	}

	// The output image format is not specified in the config or as an
	// argument, so an error will be reported.
	rc, err := ValidateConfig(t.Context(), configPath, config, false, options)
	assert.ErrorContains(t, err, "output image format must be specified")

	// Pass the output image format only in the config.
	config.Output.Image.Format = outputImageFormatAsConfig

	// The output image file should be set to the value in the config.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFormat, outputImageFormatAsConfig)

	// Pass the output image format only as an argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	options.OutputImageFormat = outputImageFormatAsArg

	// The output image file should be set to the value passed as an
	// argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFormat, outputImageFormatAsArg)

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Format = outputImageFormatAsConfig

	// The output image file should be set to the value passed as an
	// argument.
	rc, err = ValidateConfig(t.Context(), configPath, config, false, options)
	assert.NoError(t, err)
	assert.Equal(t, rc.OutputImageFormat, outputImageFormatAsArg)
}

func TestConvertImageToRawFromVhdCurrentSize(t *testing.T) {
	testConvertImageToRawSuccess(t, "TestConvertImageToRawFromVhdCurrentSize",
		[]string{"-f", "vpc", "-o", "force_size=on,subformat=fixed"},
		imagecustomizerapi.ImageFormatTypeVhd)
}

func TestConvertImageToRawFromVhdDiskGeometry(t *testing.T) {
	_, _, err := testConvertImageToRawHelper(t, "TestConvertImageToRawFromVhdDiskGeometry",
		[]string{"-f", "vpc", "-o", "force_size=off,subformat=fixed"}, 50*diskutils.MiB)
	assert.ErrorContains(t, err, "rejecting VHD file that uses 'Disk Geometry' based size")
}

func TestConvertImageToRawFromVhdx(t *testing.T) {
	testConvertImageToRawSuccess(t, "TestConvertImageToRawFromVhdx",
		[]string{"-f", "vhdx"},
		imagecustomizerapi.ImageFormatTypeVhdx)
}

func testConvertImageToRawHelper(t *testing.T, testName string, qemuImgArgs []string, diskSize int64,
) (string, imagecustomizerapi.ImageFormatType, error) {
	qemuimgExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !qemuimgExists {
		t.Skip("The 'qemu-img' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, testName)
	testImageFile := filepath.Join(testTempDir, "test.img")
	testRawFile := filepath.Join(testTempDir, "test.raw")

	err = os.MkdirAll(testTempDir, os.ModePerm)
	if err != nil {
		return "", "", err
	}

	args := []string{"create", testImageFile, fmt.Sprintf("%d", diskSize)}
	args = append(args, qemuImgArgs...)

	err = shell.ExecuteLive(true, "qemu-img", args...)
	if err != nil {
		return "", "", err
	}

	imageFormatType, err := convertImageToRaw(testImageFile, testRawFile)
	if err != nil {
		return "", "", err
	}

	return testRawFile, imageFormatType, nil
}

func testConvertImageToRawSuccess(t *testing.T, testName string, qemuImgArgs []string,
	expectedImageFormatType imagecustomizerapi.ImageFormatType,
) {
	diskSize := int64(50 * diskutils.MiB)

	testRawFile, imageFormatType, err := testConvertImageToRawHelper(t, testName, qemuImgArgs, diskSize)
	assert.NoError(t, err)
	assert.Equal(t, expectedImageFormatType, imageFormatType)

	testRawFileStat, err := os.Stat(testRawFile)
	assert.NoError(t, err)
	assert.Equal(t, int64(diskSize), testRawFileStat.Size())
}

func TestCustomizeImageBaseImageMissing(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	qemuimgExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !qemuimgExists {
		t.Skip("The 'qemu-img' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageBaseImageMissing")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-config.yaml")
	baseImage := filepath.Join(testTmpDir, "missing.qcow2")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "no such file or directory")
}

func TestCustomizeImageBaseImageInvalid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	qemuimgExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !qemuimgExists {
		t.Skip("The 'qemu-img' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageBaseImageInvalid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "partitions-config.yaml")
	baseImage := configFile
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath,
		"raw", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to open image file:")
	assert.ErrorContains(t, err, "image does not contain a partition table")
}

func checkFileType(t *testing.T, filePath string, expectedFileType string) {
	fileType, err := testutils.GetImageFileType(filePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedFileType, fileType)
}
