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

func ValidateIsoPhase1(t *testing.T, testTempDir, pxeUrlBase, outImageFilePath string) {
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

	// Check for the copied a.txt file.
	aOrigPath := filepath.Join(testDir, "files/a.txt")
	aIsoPath := filepath.Join(isoMountDir, "a.txt")
	verifyFileContentsSame(t, aOrigPath, aIsoPath)

	// Ensure grub.cfg file has the extra kernel command-line args.
	grubCfgFilePath := filepath.Join(isoMountDir, "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, "linux.* rd.info ", grubCfgContents)

	// Check the saved-configs.yaml file.
	savedConfigsFilePath := filepath.Join(isoMountDir, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	assert.NoErrorf(t, err, "read (%s) file", savedConfigsFilePath)
	expectedKernelArgs := []string{"rd.info"}
	assert.Equal(t, expectedKernelArgs, savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)

	pxeArtifactsPathVhdxToIso := ""
	if baseImageVersionDefault != baseImageVersionAzl2 {
		pxeArtifactsPathVhdxToIso = filepath.Join(testTempDir, "pxe-artifacts-vhdx-to-iso")
	}

	VerifyPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(outImageFilePath), isoMountDir, pxeUrlBase, pxeArtifactsPathVhdxToIso)

	err = isoImageMount.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	err = isoImageLoopDevice.CleanClose()
	if !assert.NoError(t, err) {
		return
	}
}

func ValidateIsoPhase2(t *testing.T, testTempDir, pxeUrlBase, outImageFilePath string) {
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

	// Check that the a.txt stayed around.
	aOrigPath := filepath.Join(testDir, "files/a.txt")
	aIsoPath := filepath.Join(isoMountDir, "a.txt")
	verifyFileContentsSame(t, aOrigPath, aIsoPath)

	// Check for copied b.txt file.
	bOrigPath := filepath.Join(testDir, "files/b.txt")
	b1IsoPath := filepath.Join(isoMountDir, "b1.txt")
	b2IsoPath := filepath.Join(isoMountDir, "b2.txt")
	verifyFileContentsSame(t, bOrigPath, b1IsoPath)
	verifyFileContentsSame(t, bOrigPath, b2IsoPath)
	verifyFilePermissions(t, os.FileMode(0600), b2IsoPath)

	// Ensure grub.cfg file has the extra kernel command-line args from both runs.
	grubCfgFilePath := filepath.Join(isoMountDir, "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)

	grubCfgContents, err = file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, "linux.* rd.info ", grubCfgContents)
	assert.Regexp(t, "linux.* rd.debug ", grubCfgContents)

	// Check the iso-kernel-args.txt file.
	savedConfigsFilePath := filepath.Join(isoMountDir, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	assert.NoErrorf(t, err, "read (%s) file", savedConfigsFilePath)
	assert.Equal(t, []string{"rd.info", "rd.debug"}, savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)

	pxeArtifactsPathIsoToIso := ""
	if baseImageVersionDefault != baseImageVersionAzl2 {
		pxeArtifactsPathIsoToIso = filepath.Join(testTempDir, "pxe-artifacts-iso-to-iso")
	}

	VerifyPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(outImageFilePath), isoMountDir, "http://my-pxe-server-2/", pxeArtifactsPathIsoToIso)
}


// Tests:
// - vhdx to ISO, with OS changes, and PXE image base URL.
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

	ValidateIsoPhase1(t, testTempDir, "http://my-pxe-server-1/", outImageFilePath)

	// Customize ISO to ISO, with no OS changes.
	configFile = filepath.Join(testDir, "liveos-bootstrapped-no-os-changes.yaml")

	err = CustomizeImageWithConfigFile(buildDir, configFile, outImageFilePath, nil, outImageFilePath, "iso", false /*useBaseImageRpmRepos*/)
	assert.NoError(t, err)

	ValidateIsoPhase2(t, testTempDir, "http://my-pxe-server-2/", outImageFilePath)
}

func VerifyPXEArtifacts(t *testing.T, packageInfo *PackageVersionInformation, outImageFileName, isoMountDir string,
	pxeBaseUrl string, pxeArtifactsPathIsoToIso string) {

	pxeKernelIpArg := "linux.* ip=dhcp "

	pxeImageFileUrl, err := url.JoinPath(pxeBaseUrl, outImageFileName)
	assert.NoError(t, err)

	pxeKernelRootArg := "linux.* root=live:" + pxeImageFileUrl
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, "/", "\\/")
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, ":", "\\:")

	// Check if PXE support is present in the Dracut package version in use.
	err = verifyDracutPXESupport(packageInfo)
	if err != nil {
		// If there is not PXE support, return
		logger.Log.Infof("PXE is not supported for this Dracut version - skipping validation")
		return
	}

	// Ensure grub-pxe.cfg file exists and has the pxe-specific command-line args.
	pxeGrubCfgFilePath := filepath.Join(isoMountDir, "/boot/grub2/grub-pxe.cfg")
	pxeGrubCfgContents, err := file.Read(pxeGrubCfgFilePath)
	assert.NoError(t, err, "read grub-pxe.cfg file")
	assert.Regexp(t, pxeKernelIpArg, pxeGrubCfgContents)
	assert.Regexp(t, pxeKernelRootArg, pxeGrubCfgContents)

	exportedPxeGrubCfgFilePath := filepath.Join(pxeArtifactsPathIsoToIso, "boot/grub2/grub.cfg")
	exportedPxeGrubCfgContents, err := file.Read(exportedPxeGrubCfgFilePath)
	assert.NoError(t, err, "read pxe grub.cfg file")
	assert.Equal(t, pxeGrubCfgContents, exportedPxeGrubCfgContents)
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
