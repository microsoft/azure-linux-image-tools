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
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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
		"vhd", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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
		"vhd-fixed", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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
		"vhdx", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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
	err := validateConfig(testDir, config, "" /*inputImageFile*/, nil, "./out/image.vhdx", "vhdx", true, "")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "input image file must be specified")
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

	inputImageFile := inputImageFileReal
	rpmSources := []string{}
	outputImageFile := "out/image.vhdx"
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// The input image file can be specified as an argument without being specified in the config.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	inputImageFile = inputImageFileRealRelativeCwd

	// The input image file specified as an argument can be relative to the current working directory.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	inputImageFile = inputImageFileFake

	// The input image file, specified as an argument, must be a file.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")

	inputImageFile = ""
	config.Input.Image.Path = inputImageFileReal

	// The input image file can be specified in the config without being specified as an argument.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileRealRelativeConfig

	// The input image file specified in the config can be relative to the bash config path.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileFake

	// The input image file, specified in the config, must be a file.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")

	inputImageFile = inputImageFileReal
	config.Input.Image.Path = inputImageFileReal

	// The input image file can be specified both as an argument and in the config.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileFake

	// The input image file can even be invalid in the config if it is specified as an argument.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
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
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "testimages/empty.vhdx",
			},
		},
	}, "", nil, "./out/image.vhdx", "vhdx", true, "")
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
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "testimages/empty.vhdx",
			},
		},
	}, "", nil, "./out/image.vhdx", "vhdx", true, "")
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
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "testimages/empty.vhdx",
			},
		},
	}, "", nil, "./out/image.vhdx", "vhdx", true, "")
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
	inputImageFile := ""
	rpmSources := []string{}
	outputImageFile := ""
	outputImageFormat := string(imagecustomizerapi.ImageFormatTypeNone)
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Test that the output is being validated in validateConfig by triggering an error in validateOutput.
	err := validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "output image file must be specified")
}

func TestValidateOutput_AcceptsValidPaths(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	buildDir := filepath.Join(tmpDir, "TestValidateOutput_AcceptsValidPaths")
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
	inputImageFile := ""
	rpmSources := []string{}

	outputImageDir := filepath.Join(buildDir, "out")
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

	outputImageFile := outputImageFileNew
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// The output image file can be sepcified as an argument without being in specified the config.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNewRelativeCwd

	// The output image file can be specified as an argument relative to the current working directory.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageDir

	// The output image file, specified as an argument, must not be a directory.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageDirRelativeCwd

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageFileExists

	// The output image file, specified as an argument, may be a file that already exists.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileExistsRelativeCwd

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = ""
	config.Output.Image.Path = outputImageFileNew

	// The output image file cab be specified in the config without being specified as an argument.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileNewRelativeConfig

	// The output image file can be specified in the config relative to the base config path.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file, specified in the config, must not be a directory.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageDirRelativeConfig

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageFileExists

	// The output image file, specified in the config, may be a file that already exists.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileExistsRelativeConfig

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNew
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified both as an argument and in the config.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file can even be invalid in the config if it is specified as an argument.
	err = validateConfig(baseConfigPath, config, inputImageFile, rpmSources, outputImageFile, outputImageFormat,
		useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
}

func TestCustomizeImage_InputImageFileSelection(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_InputImageFileSelection")
	baseConfigPath := buildDir
	config := &imagecustomizerapi.Config{}

	inputImageFileFake := filepath.Join(buildDir, "doesnotexist.xxx")
	inputImageFileReal := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	inputImageFile := inputImageFileReal
	rpmSources := []string{}
	outputImageFile := filepath.Join(buildDir, "image.vhd")
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the input image file only through the argument.
	err := CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileReal
	inputImageFile = ""

	// Pass the input image file only through the config.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	inputImageFile = inputImageFileReal
	config.Input.Image.Path = inputImageFileFake

	// Pass the input image file through both the config and the argument. The config's Path is ignored, so even though
	// it doesn't exist, there will be no error.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)
}

func TestCustomizeImage_InputImageFileAsRelativePath(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_InputImageFileAsRelativePathOnCommandLine")
	baseConfigPath := buildDir
	config := &imagecustomizerapi.Config{}

	inputImageFileAbsoluteFake := filepath.Join(buildDir, "doesnotexist.xxx")
	inputImageFileAbsoluteReal := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

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
	outputImageFile := filepath.Join(buildDir, "image.vhd")
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the input image file relative to the current working directory through the argument. This works because
	// paths on the command-line are expected to be relative to the current working directory.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	inputImageFile = inputImageFileRelativeToCwdFake

	// The same as above but for the fake path. This fails because the file does not exist.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")
	assert.NoFileExists(t, outputImageFile)

	config.Input.Image.Path = inputImageFileRelativeToConfigReal
	inputImageFile = ""

	// Pass the input image file relative to the config file through the config. This works because paths in the config
	// as expected to be relative to the config file.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	config.Input.Image.Path = inputImageFileRelativeToConfigFake

	// The same as above but for the fake path. This fails because the file does not exist.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "doesnotexist.xxx: no such file or directory")
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

	t.Logf("%s", grub2ConfigFile)

	linuxCommandLineRegex, err := regexp.Compile(`linux .* console=tty0 console=ttyS0 `)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, linuxCommandLineRegex.Match(grub2ConfigFile))
}

func TestCustomizeImage_OutputImageFileSelection(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFileSelection")
	baseConfigPath := buildDir
	config := &imagecustomizerapi.Config{}
	inputImageFile := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)
	rpmSources := []string{}

	outputImageFileAsArgument := filepath.Join(buildDir, "image-as-arg.vhd")
	outputImageFilePathAsConfig := filepath.Join(buildDir, "image-as-config.vhd")

	outputImageFile := outputImageFileAsArgument
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	useBaseImageRpmRepos := false
	packageSnapshotTime := ""

	// Pass the output image file only through the argument.
	err := CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAsArgument)
	err = os.Remove(outputImageFileAsArgument)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFilePathAsConfig
	outputImageFile = ""

	// Pass the output image file only through the config.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, "",
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFilePathAsConfig)
	err = os.Remove(outputImageFilePathAsConfig)
	assert.NoError(t, err)

	config.Output.Image.Path = buildDir
	outputImageFile = outputImageFileAsArgument

	// Pass the output image file through both the config and the argument. The config's Path is ignored, so even though
	// it is a directory, there will be no error.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAsArgument)
	assert.NoFileExists(t, outputImageFilePathAsConfig)
	err = os.Remove(outputImageFileAsArgument)
	assert.NoError(t, err)
}

func TestCustomizeImage_OutputImageFileAsRelativePath(t *testing.T) {
	buildDir := filepath.Join(tmpDir, "TestCustomizeImage_OutputImageFileAsRelativePathOnCommandLine")
	baseConfigPath := buildDir
	config := &imagecustomizerapi.Config{}
	inputImageFile := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)
	rpmSources := []string{}

	outputImageFileAbsolute := filepath.Join(buildDir, "image.vhdx")

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
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileRelativeToConfig
	outputImageFile = ""

	// Pass the output image file relative to the config file through the config. This will create the file at the
	// absolute path.
	err = CustomizeImage(buildDir, baseConfigPath, config, inputImageFile, rpmSources, outputImageFile,
		outputImageFormat, useBaseImageRpmRepos, packageSnapshotTime)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFileAbsolute)
	err = os.Remove(outputImageFileAbsolute)
	assert.NoError(t, err)
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
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsConfig)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format only through the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", outputImageFormatAsArg,
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)
	assert.FileExists(t, outputImageFile)
	checkFileType(t, outputImageFile, outputImageFormatAsArg)

	// Clean up previous test.
	err = os.Remove(outputImageFile)
	assert.NoError(t, err)

	// Pass the output image format through both the config and the argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)
	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, "", outputImageFormatAsArg,
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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
	packageSnapshotTime := ""

	// The input image file should be set to the value in the config.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsConfig)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass the input image file only as an argument.
	config.Input.Image.Path = ""
	inputImageFile = inputImageFileAsArg

	// The input image file should be set to the value passed as an argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsArg)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass the input image file in both the config and as an argument.
	config.Input.Image.Path = inputImageFileAsConfig

	// The input image file should be set to the value passed as an argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.inputImageFile, inputImageFileAsArg)
	assert.Equal(t, ic.inputImageFormat, "vhdx")
	assert.False(t, ic.inputIsIso)

	// Pass in an ISO to test that inputIsIso is set correctly.
	inputImageFile = inputImageFileIsoAsArg
	outputImageFormat = "iso"
	outputImageFile = "out/image.iso"
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
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
	packageSnapshotTime := ""

	// The output image file is not specified in the config or as an argument, so the output image file will be empty.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, "")

	// Pass the output image file only in the config.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value in the config.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsConfig)
	assert.Equal(t, ic.outputImageDir, buildDir)

	// Pass the output image file only as an argument.
	config.Output.Image.Path = ""
	outputImageFile = outputImageFilePathAsArg

	// The output image file should be set to the value passed as an argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
	assert.Equal(t, ic.outputImageDir, buildDir)

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Path = outputImageFilePathAsConfig

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFile, outputImageFilePathAsArg)
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
	packageSnapshotTime := ""

	// The output image format is not specified in the config or as an
	// argument, so the output image format will be empty.
	ic, err := createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatTypeNone)

	// Pass the output image format only in the config.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)

	// The output image file should be set to the value in the config.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig))

	// Pass the output image format only as an argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatTypeNone
	outputImageFormat = outputImageFormatAsArg

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
	assert.NoError(t, err)
	assert.Equal(t, ic.outputImageFormat, imagecustomizerapi.ImageFormatType(outputImageFormatAsArg))

	// Pass the output image file in both the config and as an argument.
	config.Output.Image.Format = imagecustomizerapi.ImageFormatType(outputImageFormatAsConfig)

	// The output image file should be set to the value passed as an
	// argument.
	ic, err = createImageCustomizerParameters(buildDir, inputImageFile, configPath, config, useBaseImageRpmRepos,
		rpmsSources, outputImageFormat, outputImageFile, packageSnapshotTime)
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
