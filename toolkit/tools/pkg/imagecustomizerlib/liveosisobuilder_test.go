// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func ValidateLiveOSPhase1(t *testing.T, testTempDir, outputFormat string, initramfsType imagecustomizerapi.InitramfsImageType,
	artifactsPath, pxeUrlBase, outImageFilePath string) {
	// Check for the copied a.txt file.
	aOrigPath := filepath.Join(testDir, "files/a.txt")
	aIsoPath := filepath.Join(artifactsPath, "a.txt")
	verifyFileContentsSame(t, aOrigPath, aIsoPath)

	// Ensure grub.cfg file has the extra kernel command-line args.
	grubCfgFilePath := filepath.Join(artifactsPath, "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, "linux.* rd.info ", grubCfgContents)

	// Check the saved-configs.yaml file.
	savedConfigsFilePath := filepath.Join(artifactsPath, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	assert.NoErrorf(t, err, "read (%s) file", savedConfigsFilePath)
	expectedKernelArgs := []string{"rd.info"}
	assert.Equal(t, expectedKernelArgs, savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)

	if outputFormat == "pxe" {
		if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			VerifyBootstrapPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(outImageFilePath), artifactsPath, pxeUrlBase)
		}
	}
}

func ValidateIsoPhase1(t *testing.T, testTempDir, outImageFilePath string) {
	// Attach ISO.
	isoImageLoopDevice, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageLoopDevice.Close()

	isoMountDir := filepath.Join(testTempDir, "iso-mount")
	isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), isoMountDir,
		"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageMount.Close()

	ValidateLiveOSPhase1(t, testTempDir, "iso" /*outputFormat*/, imagecustomizerapi.InitramfsImageTypeBootstrap,
		isoMountDir, "" /*pxeUrlBase*/, outImageFilePath)
}

func ValidatePxePhase1(t *testing.T, testTempDir, outImageFilePath string, initramfsType imagecustomizerapi.InitramfsImageType) {

	pxeArtifactsPath := ""
	if strings.HasSuffix(outImageFilePath, ".tar.gz") {
		pxeArtifactsPath = filepath.Join(testTempDir, "pxe-artifacts")
		logger.Log.Infof("-- debug -- unpacking (%s) to (%s)", outImageFilePath, pxeArtifactsPath)
		err := expandTarGzArchive(outImageFilePath, pxeArtifactsPath)
		if !assert.NoError(t, err) {
			return
		}
	} else {
		pxeArtifactsPath = outImageFilePath
	}
	logger.Log.Infof("-- debug -- pxe folder (%s)", pxeArtifactsPath)

	boostrapBaseUrl := ""
	boostrappedImage := ""
	if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		logger.Log.Infof("-- debug -- pxe bootstrap config")
		boostrapBaseUrl = "http://my-pxe-server-1"
		boostrappedImage = filepath.Join(pxeArtifactsPath, defaultIsoImageName)
	}

	ValidateLiveOSPhase1(t, testTempDir, "pxe" /*outputFormat*/, initramfsType, pxeArtifactsPath, boostrapBaseUrl, boostrappedImage)
}

func ValidateLiveOSPhase2(t *testing.T, testTempDir, outputFormat string, initramfsType imagecustomizerapi.InitramfsImageType,
	artifactsPath, pxeUrlBase, outImageFilePath string) {
	// Check that the a.txt stayed around.
	aOrigPath := filepath.Join(testDir, "files/a.txt")
	aIsoPath := filepath.Join(artifactsPath, "a.txt")
	verifyFileContentsSame(t, aOrigPath, aIsoPath)

	// Check for copied b.txt file.
	bOrigPath := filepath.Join(testDir, "files/b.txt")
	b1IsoPath := filepath.Join(artifactsPath, "b1.txt")
	b2IsoPath := filepath.Join(artifactsPath, "b2.txt")
	verifyFileContentsSame(t, bOrigPath, b1IsoPath)
	verifyFileContentsSame(t, bOrigPath, b2IsoPath)
	verifyFilePermissions(t, os.FileMode(0600), b2IsoPath)

	// Ensure grub.cfg file has the extra kernel command-line args from both runs.
	grubCfgFilePath := filepath.Join(artifactsPath, "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)

	grubCfgContents, err = file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, "linux.* rd.info ", grubCfgContents)
	assert.Regexp(t, "linux.* rd.debug ", grubCfgContents)

	// Check the iso-kernel-args.txt file.
	savedConfigsFilePath := filepath.Join(artifactsPath, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	assert.NoErrorf(t, err, "read (%s) file", savedConfigsFilePath)
	assert.Equal(t, []string{"rd.info", "rd.debug"}, savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)

	if outputFormat == "pxe" {
		if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			VerifyBootstrapPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(outImageFilePath), artifactsPath, "http://my-pxe-server-2/")
		}
	}
}

func ValidateIsoPhase2(t *testing.T, testTempDir, outImageFilePath string) {
	// Attach ISO.
	isoImageLoopDevice, err := safeloopback.NewLoopback(outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageLoopDevice.Close()

	isoMountDir := filepath.Join(testTempDir, "iso-mount")
	isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), isoMountDir,
		"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageMount.Close()

	ValidateLiveOSPhase2(t, testTempDir, "iso" /*outputFormat*/, imagecustomizerapi.InitramfsImageTypeBootstrap,
		isoMountDir, "" /*pxeUrlBase*/, outImageFilePath)
}

func VerifyBootstrapPXEArtifacts(t *testing.T, packageInfo *PackageVersionInformation, outImageFileName, isoMountDir string,
	pxeBaseUrl string) {

	pxeKernelIpArg := "linux.* ip=dhcp "

	pxeImageFileUrl, err := url.JoinPath(pxeBaseUrl, outImageFileName)
	assert.NoError(t, err)

	pxeKernelRootArg := "linux.* root=live:" + pxeImageFileUrl
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, "/", "\\/")
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, ":", "\\:")

	logger.Log.Infof("-- debug -- looking for (%s)", pxeKernelRootArg)

	// Check if PXE support is present in the Dracut package version in use.
	err = verifyDracutPXESupport(packageInfo)
	if err != nil {
		// If there is not PXE support, return
		logger.Log.Infof("PXE is not supported for this Dracut version - skipping validation")
		return
	}

	// Ensure grub-pxe.cfg file exists and has the pxe-specific command-line args.
	pxeGrubCfgFilePath := filepath.Join(isoMountDir, "/boot/grub2/grub.cfg")
	pxeGrubCfgContents, err := file.Read(pxeGrubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, pxeKernelIpArg, pxeGrubCfgContents)
	assert.Regexp(t, pxeKernelRootArg, pxeGrubCfgContents)
}

// Tests:
// - vhdx to ISO, with OS changes
// - ISO to ISO, with no OS changes.
// - .iso.Kernel command-line arg append.
// - .iso.additionalFiles
func TestCustomizeImageLiveCd1(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveCd1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFileName := "image.iso"
	outImageFilePath := filepath.Join(testTempDir, outImageFileName)

	configFile := filepath.Join(testDir, "liveos-bootstrapped-os-changes.yaml")

	// Customize vhdx to ISO, with OS changes.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "iso", true /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	ValidateIsoPhase1(t, testTempDir, outImageFilePath)

	// Customize ISO to ISO, with no OS changes.
	configFile = filepath.Join(testDir, "liveos-bootstrapped-no-os-changes.yaml")

	err = CustomizeImageWithConfigFile(buildDir, configFile, outImageFilePath, nil, outImageFilePath, "iso", false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	ValidateIsoPhase2(t, testTempDir, outImageFilePath)
}

// Tests:
// - vhdx to PXE, with OS changes, boostrap initramfs, and PXE image base URL.
// - .iso.Kernel command-line arg append.
// - .iso.additionalFiles
func TestCustomizeImagePxe1(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImagePxe1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFileName := "pxe-artifacts.tar.gz"
	outImageFilePath := filepath.Join(testTempDir, outImageFileName)

	configFile := filepath.Join(testDir, "liveos-bootstrapped-os-changes.yaml")

	// Customize vhdx to ISO, with OS changes.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "pxe", true /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	ValidatePxePhase1(t, testTempDir, outImageFilePath, imagecustomizerapi.InitramfsImageTypeBootstrap)
}

// Tests:
// - vhdx to PXE, with no OS changes, full OS initramfs, and PXE image base URL.
// - .iso.Kernel command-line arg append.
// - .iso.additionalFiles
func TestCustomizeImagePxe2(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImagePxe2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFileName := "pxe-artifacts.tar.gz"
	outImageFilePath := filepath.Join(testTempDir, outImageFileName)

	configFile := filepath.Join(testDir, "liveos-full-os-no-os-changes.yaml")

	// Customize vhdx to ISO, with OS changes.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "pxe", true /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	ValidatePxePhase1(t, testTempDir, outImageFilePath, imagecustomizerapi.InitramfsImageTypeFullOS)
}

// Tests:
// - vhdx to ISO, with no OS changes.
// - ISO to ISO, with OS changes.
func TestCustomizeImageLiveCd2(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveCd2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	outIsoFilePath := filepath.Join(testTempDir, "image.iso")

	// Customize vhdx with ISO prereqs.
	configFile := filepath.Join(testDir, "iso-os-prereqs-config.yaml")
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	// Customize image to ISO, with no OS changes.
	config := imagecustomizerapi.Config{
		Iso: &imagecustomizerapi.Iso{},
	}
	err = CustomizeImage(buildDir, testDir, &config, outImageFilePath, nil, outIsoFilePath, "iso",
		false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	// Customize ISO to ISO, with OS changes.
	configFile = filepath.Join(testDir, "addfiles-config.yaml")
	err = CustomizeImageWithConfigFile(buildDir, configFile, outIsoFilePath, nil, outIsoFilePath, "iso",
		true /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	// Attach ISO.
	isoImageLoopDevice, err := safeloopback.NewLoopback(outIsoFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageLoopDevice.Close()

	isoMountDir := filepath.Join(testTempDir, "iso-mount")
	isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), isoMountDir,
		"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
	if !assert.NoError(t, err) {
		return
	}
	defer isoImageMount.Close()

	// Attach squashfs file.
	squashfsPath := filepath.Join(isoMountDir, liveOSDir, liveOSImage)
	squashfsLoopDevice, err := safeloopback.NewLoopback(squashfsPath)
	if !assert.NoError(t, err) {
		return
	}
	defer squashfsLoopDevice.Close()

	squashfsMountDir := filepath.Join(testTempDir, "iso-squashfs")
	squashfsMount, err := safemount.NewMount(squashfsLoopDevice.DevicePath(), squashfsMountDir,
		"squashfs" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
	if !assert.NoError(t, err) {
		return
	}
	defer squashfsMount.Close()

	// Check that a.txt is in the squashfs file.
	aOrigPath := filepath.Join(testDir, "files/a.txt")
	aIsoPath := filepath.Join(squashfsMountDir, "/mnt/a/a.txt")
	verifyFileContentsSame(t, aOrigPath, aIsoPath)
}

func TestCustomizeImageLiveCdIsoNoShimEfi(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {

		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageLiveCdIsoNoShimEfi(t, "TestCustomizeImageLiveCdIsoNoShimEfi"+string(version),
				version)
		})

	}
}

func testCustomizeImageLiveCdIsoNoShimEfi(t *testing.T, testName string, version baseImageVersion) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, version)

	buildDir := filepath.Join(tmpDir, testName)
	outImageFilePath := filepath.Join(buildDir, "image.iso")
	shimPackage := "shim"

	// For arm64 and baseImageVersionAzl2, the shim package is shim-unsigned.
	if runtime.GOARCH == "arm64" && version == baseImageVersionAzl2 {
		shimPackage = "shim-unsigned"
	}

	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Remove: []string{
					shimPackage,
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the boot efi file")
}

func TestCustomizeImageLiveCdIsoNoGrubEfi(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImageLiveCdIso")
	outImageFilePath := filepath.Join(buildDir, "image.iso")

	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Remove: []string{
					"grub2-efi-binary",
				},
			},
		},
	}

	// Customize image.
	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the grub efi file")
}
