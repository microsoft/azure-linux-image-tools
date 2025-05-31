// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/tarutils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func createConfig(fileName, kernelParameter string, initramfsType imagecustomizerapi.InitramfsImageType, bootstrapFileUrl string,
	enableOsConfig, bootstrapPrereqs bool, selinuxMode imagecustomizerapi.SELinuxMode) *imagecustomizerapi.Config {

	boostrapRequiredPkgs := []string{}
	if bootstrapPrereqs {
		boostrapRequiredPkgs = []string{
			"squashfs-tools",
			"tar",
			"device-mapper",
			"curl",
		}
	}

	config := imagecustomizerapi.Config{
		Iso: &imagecustomizerapi.Iso{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      filepath.Join("files/", fileName),
					Destination: filepath.Join("/", fileName),
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
			SELinux: imagecustomizerapi.SELinux{
				Mode: selinuxMode,
			},
			Packages: imagecustomizerapi.Packages{
				Install: boostrapRequiredPkgs,
			},
		}
	}

	return &config
}

func ValidateLiveOSContent(t *testing.T, config *imagecustomizerapi.Config, testTempDir, outputFormat string, artifactsPath, bootstrappedImage string) {

	// Check for the copied a.txt file.
	var additionalFiles imagecustomizerapi.AdditionalFileList
	var extraCommandLineParameters []string
	var initramfsType imagecustomizerapi.InitramfsImageType
	var pxeUrlBase string

	if outputFormat == "iso" {
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
		aOrigPath := filepath.Join(testDir, additionalFile.Source)
		aIsoPath := filepath.Join(artifactsPath, additionalFile.Destination)
		verifyFileContentsSame(t, aOrigPath, aIsoPath)
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
		assert.Contains(t, savedConfigs.Iso.KernelCommandLine.ExtraCommandLine, extraCommandLineParameter)
	}

	rootfsImagePath := ""
	if outputFormat == "iso" {
		rootfsImagePath = filepath.Join(artifactsPath, liveOSDir, liveOSImage)
	} else {
		rootfsImagePath = filepath.Join(artifactsPath, defaultIsoImageName)
	}

	rootfsImageExists, err := file.PathExists(rootfsImagePath)
	assert.NoErrorf(t, err, "check if (%s) rootfsImagePath exists", rootfsImagePath)

	switch initramfsType {
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		assert.Equal(t, rootfsImageExists, true)
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		assert.Equal(t, rootfsImageExists, false)
	}

	if outputFormat == "pxe" {
		if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			VerifyBootstrapPXEArtifacts(t, savedConfigs.OS.DracutPackageInfo, filepath.Base(bootstrappedImage), artifactsPath, pxeUrlBase)
		}
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

	ValidateLiveOSContent(t, config, testTempDir, "iso" /*outputFormat*/, isoMountDir, "" /*bootstrappedImage*/)
}

func ValidatePxeContent(t *testing.T, config *imagecustomizerapi.Config, testTempDir, outImageFilePath string) {

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

	boostrappedImage := ""
	if config.Pxe != nil && config.Pxe.InitramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		boostrappedImage = filepath.Join(pxeArtifactsPath, defaultIsoImageName)
	}

	ValidateLiveOSContent(t, config, testTempDir, "pxe" /*outputFormat*/, pxeArtifactsPath, boostrappedImage)
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
// - vhdx {raw}        to ISO {bootstrap}    , with selinux enforcing + bootstrap rpereqs
// - ISO  {bootstrap}  to ISO {bootstrap}    , with no OS changes
func TestCustomizeImageLiveOSIso1(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSIso1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	// vhdx {raw} to ISO {bootstrap}, selinux enforcing + bootstrap rpereqs
	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)

	// ISO  {bootstrap} to ISO {bootstrap}, with no OS changes
	configB := createConfig("b.txt", "rd.debug", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		false /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDefault)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath, "iso",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)
}

// Tests: raw -> initramfs full -> initramfs full (RFF)
// - vhdx {raw}        to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {full-os}      , with selinux disabled
func TestCustomizeImageLiveOSInitramfsRFF(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfsRFF")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os}, with selinux disabled
	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath, "iso",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)

	// ISO  {full-os} to ISO {full-os}, with selinux disabled
	configB := createConfig("b.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)

	assert.NoError(t, err)

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)
}

// Tests: raw -> initramfs full -> initramfs bootstrap (RFB)
// - vhdx {raw}        to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {boostrap}     , with selinux enforcing + bootstrap rpereqs
func TestCustomizeImageLiveOSInitramfsRFB(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfsRFB")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os} , with selinux disabled
	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)

	// - ISO {full-os} to ISO {boostrap}, with selinux enforcing
	configB := createConfig("b.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)
}

// Tests: raw -> initramfs bootstrap -> initramfs full-os (RBF)
// - vhdx {raw}        to ISO {boostrap}    , with selinux enforcing + bootstrap prereqs
// - ISO  {boostrap}   to ISO {full-os}     , with selinux disabled
func TestCustomizeImageLiveOSInitramfsRBF(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfsRBF")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {boostrap}, with selinux enforcing + bootstrap preres
	configA := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap, "", /*pxe url*/
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	err := CustomizeImage(buildDir, testDir, configA, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath)

	// - ISO {bootstrap} to ISO {full-os}, with selinux disabled
	configB := createConfig("b.txt", "rd.debug", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err = CustomizeImage(buildDir, testDir, configB, outImageFilePath, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath)
}

// Tests:
// - vhdx {raw} to PXE {bootstrap}, with selinux enforcing
func TestCustomizeImageLiveOSPxe1(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe1")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")
	pxeBootstrapUrl := "http://my-pxe-server-1" + defaultIsoImageName

	config := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap, pxeBootstrapUrl,
		true /*enable os config*/, true /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeEnforcing)

	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath, "pxe",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidatePxeContent(t, config, testTempDir, outImageFilePath)
}

// Tests:
// - vhdx {raw} to PXE {full-os}, with selinux disabled
func TestCustomizeImageLiveOSPxe2(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")

	config := createConfig("a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS, "", /*pxe url*/
		true /*enable os config*/, false /*bootstrap prereqs*/, imagecustomizerapi.SELinuxModeDisabled)

	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath, "pxe",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	ValidatePxeContent(t, config, testTempDir, outImageFilePath)
}

// Tests:
// - vhdx to ISO, with no OS changes.
// - ISO to ISO, with OS changes.
func TestCustomizeImageLiveOSIso2(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSIso2")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	outIsoFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// Customize vhdx with ISO prereqs.
	configFile := filepath.Join(testDir, "iso-os-prereqs-config.yaml")
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	// Customize image to ISO, with no OS changes.
	config := imagecustomizerapi.Config{
		Iso: &imagecustomizerapi.Iso{},
	}
	err = CustomizeImage(buildDir, testDir, &config, outImageFilePath, nil, outIsoFilePath, "iso",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.NoError(t, err)

	// Customize ISO to ISO, with OS changes.
	configFile = filepath.Join(testDir, "addfiles-config.yaml")
	err = CustomizeImageWithConfigFile(buildDir, configFile, outIsoFilePath, nil, outIsoFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
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

func TestCustomizeImageLiveOSIsoNoShimEfi(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {

		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageLiveOSIsoNoShimEfi(t, "TestCustomizeImageLiveCdIsoNoShimEfi"+string(version),
				version)
		})

	}
}

func testCustomizeImageLiveOSIsoNoShimEfi(t *testing.T, testName string, version baseImageVersion) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, version)

	buildDir := filepath.Join(tmpDir, testName)
	outImageFilePath := filepath.Join(buildDir, defaultIsoImageName)
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
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the boot efi file")
}

func TestCustomizeImageLiveOSIsoNoGrubEfi(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

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
	err := CustomizeImage(buildDir, testDir, config, baseImage, nil, outImageFilePath, "iso",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the grub efi file")
}
