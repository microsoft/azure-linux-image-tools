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
		"vhd", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false, /*enableShrinkFilesystems*/
		false, false, false, "", nil, "", nil, "", "", "", "")
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
		"vhd", "", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false, /*enableShrinkFilesystems*/
		false, false, false, "", nil, "", nil, "", "", "", "")
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
		"vhd-fixed", "", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false, /*enableShrinkFilesystems*/
		false, false, false, "", nil, "", nil, "", "", "", "")
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
		"vhdx", "", "" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false, /*enableShrinkFilesystems*/
		false, false, false, "", nil, "", nil, "", "", "", "")
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

func TestValidateConfigValidAdditionalFiles(t *testing.T) {
	err := validateConfig(testDir, &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "files/a.txt",
					Destination: "/a.txt",
				},
			},
		}}, nil, true)
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
		}}, nil, true)
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
		}}, nil, true)
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

	err = CustomizeImage(buildDir, buildDir, config, baseImage, nil, outImageFilePath, "raw", "",
		"" /*outputPXEArtifactsDir*/, false /*useBaseImageRpmRepos*/, false, /*enableShrinkFilesystems*/
		false, false, false, "", nil, "", nil, "", "", "", "")
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
