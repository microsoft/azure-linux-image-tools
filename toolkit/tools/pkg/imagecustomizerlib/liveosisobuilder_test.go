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
	"github.com/microsoft/azurelinux/toolkit/tools/internal/initrdutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/tarutils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func createConfig(fileName, kernelParameter string, initramfsType imagecustomizerapi.InitramfsImageType, bootstrapFileUrl string,
	enableOsConfig, bootstrapPrereqs bool, selinuxMode imagecustomizerapi.SELinuxMode) *imagecustomizerapi.Config {
	bootstrapRequiredPkgs := []string{}
	if bootstrapPrereqs {
		bootstrapRequiredPkgs = append(bootstrapRequiredPkgs, "squashfs-tools", "tar", "device-mapper", "curl")
	}
	if selinuxMode != imagecustomizerapi.SELinuxModeDisabled {
		bootstrapRequiredPkgs = append(bootstrapRequiredPkgs, "selinux-policy")
	}

	perms0o644 := imagecustomizerapi.FilePermissions(0o644)

	config := imagecustomizerapi.Config{
		Iso: &imagecustomizerapi.Iso{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      filepath.Join("files/", fileName),
					Destination: filepath.Join("/", fileName),
					Permissions: &perms0o644,
				},
			},
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{kernelParameter},
			},
			InitramfsType: initramfsType,
		},
		Pxe: &imagecustomizerapi.Pxe{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      filepath.Join("files/", fileName),
					Destination: filepath.Join("/", fileName),
					Permissions: &perms0o644,
				},
			},
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{kernelParameter},
			},
			InitramfsType:    initramfsType,
			BootstrapFileUrl: bootstrapFileUrl,
		},
	}

	if enableOsConfig {
		config.OS = &imagecustomizerapi.OS{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      filepath.Join("files/", fileName),
					Destination: filepath.Join("/", fileName),
					// Need to ensure the packaged full OS supports setuid, setgid, and sticky bits.
					Permissions: &perms0o644,
				},
			},
			SELinux: imagecustomizerapi.SELinux{
				Mode: selinuxMode,
			},
			Packages: imagecustomizerapi.Packages{
				Install: bootstrapRequiredPkgs,
			},
		}
	}

	return &config
}

func VerifyBootstrappedImageExists(t *testing.T, initramfsType imagecustomizerapi.InitramfsImageType, bootstrappedImagePath string) {
	bootstrapImageExists, err := file.PathExists(bootstrappedImagePath)
	assert.NoErrorf(t, err, "check if (%s) bootstrappedImagePath exists", bootstrappedImagePath)

	switch initramfsType {
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		assert.Equal(t, bootstrapImageExists, true)
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		assert.Equal(t, bootstrapImageExists, false)
	}
}

func ValidateLiveOSContent(t *testing.T, outputFormat imagecustomizerapi.ImageFormatType, config *imagecustomizerapi.Config, testTempDir string, artifactsPath, bootstrappedImage string) {
	var additionalFiles imagecustomizerapi.AdditionalFileList
	var extraCommandLineParameters []string
	var initramfsType imagecustomizerapi.InitramfsImageType
	var pxeUrlBase string

	if outputFormat == imagecustomizerapi.ImageFormatTypeIso {
		additionalFiles = config.Iso.AdditionalFiles
		extraCommandLineParameters = config.Iso.KernelCommandLine.ExtraCommandLine
		initramfsType = config.Iso.InitramfsType
	} else {
		additionalFiles = config.Pxe.AdditionalFiles
		extraCommandLineParameters = config.Pxe.KernelCommandLine.ExtraCommandLine
		initramfsType = config.Pxe.InitramfsType
		pxeUrlBase = config.Pxe.BootstrapFileUrl
	}

	for _, additionalFile := range additionalFiles {
		origFilePath := filepath.Join(testDir, additionalFile.Source)
		fullOSFilePath := filepath.Join(artifactsPath, additionalFile.Destination)
		verifyFileContentsSame(t, origFilePath, fullOSFilePath)
		verifyFilePermissions(t, os.FileMode(*additionalFile.Permissions), fullOSFilePath)
	}

	// Ensure grub.cfg file has the extra kernel command-line args.
	grubCfgFilePath := filepath.Join(artifactsPath, grubCfgDir, isoGrubCfg)
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	for _, extraCommandLineParameter := range extraCommandLineParameters {
		assert.Regexp(t, "linux.* "+extraCommandLineParameter+" ", grubCfgContents)
	}

	// Check the saved-configs.yaml file.
	savedConfigsFilePath := filepath.Join(artifactsPath, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	assert.NoErrorf(t, err, "read (%s) file", savedConfigsFilePath)
	for _, extraCommandLineParameter := range extraCommandLineParameters {
		assert.Contains(t, savedConfigs.LiveOS.KernelCommandLine.ExtraCommandLine, extraCommandLineParameter)
	}

	bootstrappedImagePath := ""
	if outputFormat == imagecustomizerapi.ImageFormatTypeIso {
		// The bootstrap file is a squashfs image file
		bootstrappedImagePath = filepath.Join(artifactsPath, liveOSDir, liveOSImage)
	} else {
		// The bootstrap file is an iso that contains the squashfs file
		bootstrappedImagePath = filepath.Join(artifactsPath, defaultIsoImageName)
	}

	VerifyBootstrappedImageExists(t, initramfsType, bootstrappedImagePath)
	VerifyFullOSContents(t, testTempDir, artifactsPath, outputFormat, config.OS, bootstrappedImagePath, initramfsType)

	if outputFormat == "pxe" {
		if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			VerifyBootstrapPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(bootstrappedImage), artifactsPath, pxeUrlBase)
		}
	}
}

func VerifyFullOSContents(t *testing.T, testTempDir, artifactsPath string, outputFormat imagecustomizerapi.ImageFormatType,
	osConfig *imagecustomizerapi.OS, bootstrappedImagePath string, initramfsType imagecustomizerapi.InitramfsImageType) {
	if osConfig == nil {
		return
	}
	fullOsDir := filepath.Join(testTempDir, "full-os")

	switch initramfsType {
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		fullOSImagePath := ""
		if outputFormat == imagecustomizerapi.ImageFormatTypeIso {
			// The full OS image is the bootstrap image
			fullOSImagePath = bootstrappedImagePath
		} else {
			// The bootstrap file is an iso that contains the squashfs file
			isoImageLoopDevice, err := safeloopback.NewLoopback(bootstrappedImagePath)
			if !assert.NoError(t, err) {
				return
			}
			defer isoImageLoopDevice.Close()

			isoMountDir := filepath.Join(testTempDir, "bootstrap-iso-mount")
			isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), isoMountDir,
				"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
			if !assert.NoError(t, err) {
				return
			}
			defer isoImageMount.Close()

			fullOSImagePath = filepath.Join(isoMountDir, liveOSDir, liveOSImage)
		}

		// Attach squashfs file.
		squashfsLoopDevice, err := safeloopback.NewLoopback(fullOSImagePath)
		if !assert.NoError(t, err) {
			return
		}
		defer squashfsLoopDevice.Close()

		squashfsMount, err := safemount.NewMount(squashfsLoopDevice.DevicePath(), fullOsDir,
			"squashfs" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, true /*makeAndDelete*/)
		if !assert.NoError(t, err) {
			return
		}
		defer squashfsMount.Close()
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		fullOSImagePath := filepath.Join(artifactsPath, "boot/initrd.img")
		// Expand initrd to a folder
		err := initrdutils.CreateFolderFromInitrdImage(fullOSImagePath, fullOsDir)
		if !assert.NoError(t, err) {
			return
		}
		defer os.RemoveAll(fullOsDir)
	}

	// Check that each file is in the root file system.
	for _, additionalFile := range osConfig.AdditionalFiles {
		origFilePath := filepath.Join(testDir, additionalFile.Source)
		fullOSFilePath := filepath.Join(fullOsDir, additionalFile.Destination)
		verifyFileContentsSame(t, origFilePath, fullOSFilePath)
		verifyFilePermissions(t, os.FileMode(*additionalFile.Permissions), fullOSFilePath)
	}
}

func ValidateIsoContent(t *testing.T, config *imagecustomizerapi.Config, testTempDir string, initramfsType imagecustomizerapi.InitramfsImageType, outImageFilePath string) {
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

	ValidateLiveOSContent(t, imagecustomizerapi.ImageFormatTypeIso, config, testTempDir, isoMountDir, "" /*bootstrappedImage*/)
}

func ValidatePxeContent(t *testing.T, outputFormat imagecustomizerapi.ImageFormatType, config *imagecustomizerapi.Config, testTempDir, outImageFilePath string) {
	pxeArtifactsPath := ""
	if strings.HasSuffix(outImageFilePath, ".tar.gz") {
		pxeArtifactsPath = filepath.Join(testTempDir, "pxe-artifacts")
		err := tarutils.ExpandTarGzArchive(outImageFilePath, pxeArtifactsPath)
		if !assert.NoError(t, err) {
			return
		}
	} else {
		pxeArtifactsPath = outImageFilePath
	}

	bootstrappedImage := ""
	if config.Pxe != nil && config.Pxe.InitramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		bootstrappedImage = filepath.Join(pxeArtifactsPath, defaultIsoImageName)
	}

	ValidateLiveOSContent(t, outputFormat, config, testTempDir, pxeArtifactsPath, bootstrappedImage)
}

func VerifyBootstrapPXEArtifacts(t *testing.T, packageInfo *PackageVersionInformation, outImageFileName, isoMountDir, pxeBaseUrl string) {
	var err error
	pxeKernelIpArg := "linux.* ip=dhcp "

	pxeImageFileUrl := ""
	if strings.HasSuffix(pxeBaseUrl, ".iso") {
		pxeImageFileUrl = pxeBaseUrl
	} else {
		pxeImageFileUrl, err = url.JoinPath(pxeBaseUrl, outImageFileName)
		assert.NoError(t, err)
	}

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
	pxeGrubCfgFilePath := filepath.Join(isoMountDir, "/boot/grub2/grub.cfg")
	pxeGrubCfgContents, err := file.Read(pxeGrubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")
	assert.Regexp(t, pxeKernelIpArg, pxeGrubCfgContents)
	assert.Regexp(t, pxeKernelRootArg, pxeGrubCfgContents)
}

// Tests:
// - raw -> iso {bootstrap} -> iso {bootstrap} -> iso {full-os}
//
// - vhdx {raw}        to ISO {bootstrap}    , with selinux enforcing + bootstrap prereqs
// - ISO  {bootstrap}  to ISO {bootstrap}    , with no OS changes
// - ISO  {bootstrap}  to ISO {full-os}      , with selinux disabled
func TestCustomizeImageLiveOSInitramfs1(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// SELinux in Live OS is only supported with azl3
	selinuxMode := imagecustomizerapi.SELinuxModeEnforcing
	if baseImageInfo.Version == baseImageVersionAzl2 {
		selinuxMode = imagecustomizerapi.SELinuxModeDisabled
	}

	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, selinuxMode)

	// vhdx {raw} to ISO {bootstrap}, selinux enforcing + bootstrap prereqs
	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)

	// ISO  {bootstrap} to ISO {bootstrap}, with no OS changes
	configB := createConfig("b.txt", "rd.debug", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		false /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDefault)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)

	// - ISO {bootstrap} to ISO {full-os}, with selinux disabled
	configC := createConfig("c.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err = CustomizeImage(buildDir, testDir, configC, outImageFilePath, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configC, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)
}

// Tests:
// - raw -> iso {full-os} -> iso {full-os} -> iso {bootstrap}
//
// - vhdx {raw}        to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {bootstrap}    , with selinux enforcing + bootstrap prereqs
func TestCustomizeImageLiveOSInitramfs2(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os}, with selinux disabled
	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)

	// ISO  {full-os} to ISO {full-os}, with selinux disabled
	configB := createConfig("b.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath, string(imagecustomizerapi.ImageFormatTypeIso),
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)

	// - ISO {full-os} to ISO {bootstrap}, with selinux enforcing

	// SELinux in Live OS is only supported with azl3
	selinuxMode := imagecustomizerapi.SELinuxModeEnforcing
	if baseImageInfo.Version == baseImageVersionAzl2 {
		selinuxMode = imagecustomizerapi.SELinuxModeDisabled
	}

	configC := createConfig("c.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, selinuxMode)

	err = CustomizeImage(buildDir, testDir, configC, outImageFilePath, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configC, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)
}

// Tests:
// - vhdx {raw} to ISO {full-os}, with selinux enabled -> error
func TestCustomizeImageLiveOSInitramfs3(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs3")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os}, with selinux disabled
	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "selinux is not supported for full OS initramfs image")
}

// Tests:
// - vhdx {raw} to PXE {bootstrap}, with selinux enforcing
func TestCustomizeImageLiveOSPxe1(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("Skipping - PXE bootstrap is not supported for Azure Linux 2")
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")
	pxeBootstrapUrl := "http://my-pxe-server-1/" + defaultIsoImageName

	config := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap, pxeBootstrapUrl,
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypePxeTar), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidatePxeContent(t, imagecustomizerapi.ImageFormatTypePxeTar, config, testTempDir, outImageFilePath)
}

// Tests:
// - vhdx {raw} to PXE {full-os}, with selinux disabled
func TestCustomizeImageLiveOSPxe2(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")

	config := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypePxeTar), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	ValidatePxeContent(t, imagecustomizerapi.ImageFormatTypePxeTar, config, testTempDir, outImageFilePath)
}

func TestCustomizeImageLiveOSIsoNoShimEfi(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSIsoNoShimEfi(t, "TestCustomizeImageLiveCdIsoNoShimEfi"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageLiveOSIsoNoShimEfi(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	buildDir := filepath.Join(tmpDir, testName)
	outImageFilePath := filepath.Join(buildDir, defaultIsoImageName)
	shimPackage := "shim"

	// For arm64 and baseImageVersionAzl2, the shim package is shim-unsigned.
	if runtime.GOARCH == "arm64" && baseImageInfo.Distro == baseImageDistroAzureLinux && baseImageInfo.Version == baseImageVersionAzl2 {
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
	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the boot efi file")
}

func TestCustomizeImageLiveOSIsoNoGrubEfi(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	buildDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSIsoNoGrubEfi")
	outImageFilePath := filepath.Join(buildDir, defaultIsoImageName)

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
	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the grub efi file")
}
