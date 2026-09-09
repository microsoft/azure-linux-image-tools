// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/initrdutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/tarutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func createConfig(t *testing.T, baseImageVersion string, fileNames, kernelParameter string, initramfsType imagecustomizerapi.InitramfsImageType,
	bootstrapFileUrl string, enlargeDisk, enableOsConfig, bootstrapPrereqs, twoKernels bool, kdumpBootFiles imagecustomizerapi.KdumpBootFilesType,
	selinuxMode imagecustomizerapi.SELinuxMode, removeToolsPackages bool,
) *imagecustomizerapi.Config {
	fileNamesArray := strings.Split(fileNames, ";")

	pkgsToInstall := []string{}
	baseConfigs := []imagecustomizerapi.BaseConfig{}

	if bootstrapPrereqs {
		liveOSRequiredPackages := liveOSRequiredPackagesAzl3
		if baseImageVersion == baseImageVersionAzl4 {
			liveOSRequiredPackages = liveOSRequiredPackagesFedora
		}
		pkgsToInstall = append(pkgsToInstall, liveOSRequiredPackages...)
	}
	if selinuxMode != imagecustomizerapi.SELinuxModeDisabled {
		pkgsToInstall = append(pkgsToInstall, "selinux-policy")
	}

	if twoKernels {
		// include an old kernel
		kernelPackageName := ""
		switch baseImageVersion {
		case baseImageVersionAzl2:
			kernelPackageName = "kernel-5.15.122.1-2.cm2"
		case baseImageVersionAzl3:
			kernelPackageName = "kernel-6.6.57.1-6.azl3"
		case baseImageVersionAzl4:
			kernelPackageName = "kernel-6.18.5-1.8.azl4"
		default:
			assert.NoError(t, fmt.Errorf("undefined image version"), "unsupported distro version")
		}
		pkgsToInstall = append(pkgsToInstall, kernelPackageName)
	}

	if removeToolsPackages {
		toolsBaseConfig := ""
		switch baseImageVersion {
		case baseImageVersionAzl2, baseImageVersionAzl3:
			toolsBaseConfig = "toolsdir-azl3.yaml"
		case baseImageVersionAzl4:
			toolsBaseConfig = "toolsdir-azl4.yaml"
		default:
			assert.NoError(t, fmt.Errorf("undefined image version"), "unsupported distro version")
		}

		baseConfigs = append(baseConfigs, imagecustomizerapi.BaseConfig{Path: toolsBaseConfig})
	}

	perms0o644 := imagecustomizerapi.FilePermissions(0o644)

	config := imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureKdumpBootFiles,
			imagecustomizerapi.PreviewFeatureBaseConfigs,
		},
		BaseConfigs: baseConfigs,
		Iso: &imagecustomizerapi.Iso{
			KdumpBootFiles: &kdumpBootFiles,
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{kernelParameter},
			},
			InitramfsType: initramfsType,
		},
		Pxe: &imagecustomizerapi.Pxe{
			KdumpBootFiles: &kdumpBootFiles,
			KernelCommandLine: imagecustomizerapi.KernelCommandLine{
				ExtraCommandLine: []string{kernelParameter},
			},
			InitramfsType:    initramfsType,
			BootstrapFileUrl: bootstrapFileUrl,
		},
	}

	for _, fileName := range fileNamesArray {
		config.Iso.AdditionalFiles = append(config.Iso.AdditionalFiles, imagecustomizerapi.AdditionalFile{
			Source:      filepath.Join("files/", fileName),
			Destination: filepath.Join("/", fileName),
			Permissions: &perms0o644,
		})
		config.Pxe.AdditionalFiles = append(config.Pxe.AdditionalFiles, imagecustomizerapi.AdditionalFile{
			Source:      filepath.Join("files/", fileName),
			Destination: filepath.Join("/", fileName),
			Permissions: &perms0o644,
		})
	}

	if enlargeDisk {
		config.Storage = imagecustomizerapi.Storage{
			BootType: imagecustomizerapi.BootTypeEfi,
			Disks: []imagecustomizerapi.Disk{{
				PartitionTableType: "gpt",
				Partitions: []imagecustomizerapi.Partition{
					{
						Id: "esp",
						Size: imagecustomizerapi.PartitionSize{
							Type: imagecustomizerapi.PartitionSizeTypeExplicit,
							Size: 32 * diskutils.MiB,
						},
						Type: imagecustomizerapi.PartitionTypeESP,
					},
					{
						Id: "boot",
						Size: imagecustomizerapi.PartitionSize{
							Type: imagecustomizerapi.PartitionSizeTypeExplicit,
							Size: 512 * diskutils.MiB,
						},
					},
					{
						Id: "root",
						Size: imagecustomizerapi.PartitionSize{
							Type: imagecustomizerapi.PartitionSizeTypeExplicit,
							Size: 3 * diskutils.GiB,
						},
					},
				},
			}},
			FileSystems: []imagecustomizerapi.FileSystem{
				{
					DeviceId: "esp",
					Type:     "vfat",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/boot/efi",
					},
				},
				{
					DeviceId: "boot",
					Type:     "ext4",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/boot",
					},
				},
				{
					DeviceId: "root",
					Type:     "ext4",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/",
					},
				},
			},
		}
	}

	if enableOsConfig {
		config.OS = &imagecustomizerapi.OS{
			SELinux: imagecustomizerapi.SELinux{
				Mode: selinuxMode,
			},
			Packages: imagecustomizerapi.Packages{
				Install: pkgsToInstall,
			},
		}

		for _, fileName := range fileNamesArray {
			config.OS.AdditionalFiles = append(config.OS.AdditionalFiles, imagecustomizerapi.AdditionalFile{
				Source:      filepath.Join("files/", fileName),
				Destination: filepath.Join("/", fileName),
				Permissions: &perms0o644,
			})
		}

		if enlargeDisk {
			config.OS.BootLoader.ResetType = imagecustomizerapi.ResetBootLoaderTypeHard
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

func ValidateLiveOSContent(t *testing.T, outputFormat imagecustomizerapi.ImageFormatType, config *imagecustomizerapi.Config,
	testTempDir string, artifactsPath, bootstrappedImage string, baseImageInfo testBaseImageInfo,
) {
	var additionalFiles imagecustomizerapi.AdditionalFileList
	var extraCommandLineParameters []string
	var keepKdumpBootFiles bool
	var initramfsType imagecustomizerapi.InitramfsImageType
	var pxeUrlBase string

	if outputFormat == imagecustomizerapi.ImageFormatTypeIso {
		additionalFiles = config.Iso.AdditionalFiles
		extraCommandLineParameters = config.Iso.KernelCommandLine.ExtraCommandLine
		keepKdumpBootFiles = *config.Iso.KdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeKeep
		initramfsType = config.Iso.InitramfsType
	} else {
		additionalFiles = config.Pxe.AdditionalFiles
		extraCommandLineParameters = config.Pxe.KernelCommandLine.ExtraCommandLine
		keepKdumpBootFiles = *config.Pxe.KdumpBootFiles == imagecustomizerapi.KdumpBootFilesTypeKeep
		initramfsType = config.Pxe.InitramfsType
		pxeUrlBase = config.Pxe.BootstrapFileUrl
	}

	for _, additionalFile := range additionalFiles {
		origFilePath := filepath.Join(testDir, additionalFile.Source)
		fullOSFilePath := filepath.Join(artifactsPath, additionalFile.Destination)
		verifyFileContentsSame(t, origFilePath, fullOSFilePath)
		verifyFilePermissions(t, os.FileMode(*additionalFile.Permissions), fullOSFilePath)
	}

	// Ensure the kernel command line carries the extra kernel command-line args. Inline-grub distros keep these on
	// the grub.cfg 'linux' lines; BLS distros (Azure Linux 4.0, Fedora) keep them on the BLS entry 'options' lines.
	kernelEntryContent, kernelEntryKeyword := readLiveOSKernelEntryContent(t, artifactsPath, baseImageInfo)
	for _, extraCommandLineParameter := range extraCommandLineParameters {
		assert.Regexp(t, kernelEntryKeyword+".* "+extraCommandLineParameter+`\s`, kernelEntryContent)
	}

	// Check the saved-configs.yaml file.
	savedConfigsFilePath := filepath.Join(artifactsPath, savedConfigsDir, savedConfigsFileName)
	savedConfigs := &SavedConfigs{}
	err := imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
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
	VerifyFullOSContents(t, testTempDir, artifactsPath, outputFormat, config.OS, bootstrappedImagePath, initramfsType, keepKdumpBootFiles)

	if outputFormat == "pxe" {
		if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			VerifyBootstrapPXEArtifacts(t, filepath.Base(bootstrappedImage), artifactsPath, pxeUrlBase, baseImageInfo)
		}
	}
}

func VerifyFullOSContents(t *testing.T, testTempDir, artifactsPath string, outputFormat imagecustomizerapi.ImageFormatType,
	osConfig *imagecustomizerapi.OS, bootstrappedImagePath string, initramfsType imagecustomizerapi.InitramfsImageType,
	keepKdumpBootFiles bool,
) {
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

	additionalKdumpBootFilesExist := false
	// Check that each file is in the root file system.
	for _, additionalFile := range osConfig.AdditionalFiles {
		origFilePath := filepath.Join(testDir, additionalFile.Source)
		fullOSFilePath := filepath.Join(fullOsDir, additionalFile.Destination)
		if strings.Contains(fullOSFilePath, "kdump.img") {
			additionalKdumpBootFilesExist = additionalKdumpBootFilesExist || true
		}
		// While from an API perspective additional files in /boot are no
		// different than any other folder, we will not test them here but
		// rather under the keepKdumpBootFiles flag later.
		if !strings.HasPrefix(additionalFile.Destination, "/boot") {
			verifyFileContentsSame(t, origFilePath, fullOSFilePath)
			verifyFilePermissions(t, os.FileMode(*additionalFile.Permissions), fullOSFilePath)
		}
	}

	bootFolder := filepath.Join(fullOsDir, "/boot")
	actualBootFolderExists, err := file.PathExists(bootFolder)
	if !assert.NoError(t, err) {
		return
	}
	if keepKdumpBootFiles {
		assert.Equal(t, actualBootFolderExists, additionalKdumpBootFilesExist)
		if additionalKdumpBootFilesExist {
			for _, additionalFile := range osConfig.AdditionalFiles {
				origFilePath := filepath.Join(testDir, additionalFile.Source)
				fullOSFilePath := filepath.Join(fullOsDir, additionalFile.Destination)

				// This test expects the kdump initramfs and kernel to be the
				// only two defined under additional files uner /boot. If other
				// files are defined under /boot, the test will fail because they
				// (correctly) got removed by the Image Customizer (not being
				// part of the kdump pair). So, the failure in that case will be
				// incorrect.
				if strings.HasPrefix(additionalFile.Destination, "/boot") {
					verifyFileContentsSame(t, origFilePath, fullOSFilePath)
					verifyFilePermissions(t, os.FileMode(*additionalFile.Permissions), fullOSFilePath)
				}
			}
		}
	} else {
		assert.Equal(t, actualBootFolderExists, false)
	}
}

func ValidateIsoContent(t *testing.T, config *imagecustomizerapi.Config, testTempDir string, initramfsType imagecustomizerapi.InitramfsImageType, outImageFilePath string, baseImageInfo testBaseImageInfo) {
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

	ValidateLiveOSContent(t, imagecustomizerapi.ImageFormatTypeIso, config, testTempDir, isoMountDir, "" /*bootstrappedImage*/, baseImageInfo)
}

func ValidatePxeContent(t *testing.T, outputFormat imagecustomizerapi.ImageFormatType, config *imagecustomizerapi.Config, testTempDir, outImageFilePath string, baseImageInfo testBaseImageInfo) {
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

	ValidateLiveOSContent(t, outputFormat, config, testTempDir, pxeArtifactsPath, bootstrappedImage, baseImageInfo)
}

func VerifyBootstrapPXEArtifacts(t *testing.T, outImageFileName, isoMountDir, pxeBaseUrl string, baseImageInfo testBaseImageInfo) {
	var err error

	pxeImageFileUrl := ""
	if strings.HasSuffix(pxeBaseUrl, ".iso") {
		pxeImageFileUrl = pxeBaseUrl
	} else {
		pxeImageFileUrl, err = url.JoinPath(pxeBaseUrl, outImageFileName)
		assert.NoError(t, err)
	}

	pxeKernelEntryContent, pxeKernelEntryKeyword := readLiveOSKernelEntryContent(t, isoMountDir, baseImageInfo)

	pxeKernelIpArg := pxeKernelEntryKeyword + ".* ip=dhcp "

	pxeKernelRootArg := pxeKernelEntryKeyword + ".* root=live:" + pxeImageFileUrl
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, "/", "\\/")
	pxeKernelRootArg = strings.ReplaceAll(pxeKernelRootArg, ":", "\\:")

	assert.Regexp(t, pxeKernelIpArg, pxeKernelEntryContent)
	assert.Regexp(t, pxeKernelRootArg, pxeKernelEntryContent)
}

// readLiveOSKernelEntryContent returns the text that carries the LiveOS kernel command line under artifactsPath, along
// with the grub keyword that introduces it.
func readLiveOSKernelEntryContent(t *testing.T, artifactsPath string, baseImageInfo testBaseImageInfo) (string, string) {
	if baseImageInfo.Version == baseImageVersionAzl4 {
		blsEntriesDir := filepath.Join(artifactsPath, "boot/loader/entries")
		blsEntries, err := os.ReadDir(blsEntriesDir)
		assert.NoError(t, err, "read BLS entries directory")

		var builder strings.Builder
		entryCount := 0
		for _, entry := range blsEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}
			entryContent, readErr := file.Read(filepath.Join(blsEntriesDir, entry.Name()))
			assert.NoError(t, readErr, "read BLS entry %s", entry.Name())
			builder.WriteString(entryContent)
			builder.WriteString("\n")
			entryCount++
		}
		assert.Greater(t, entryCount, 0, "expected at least one BLS entry under %s", blsEntriesDir)
		return builder.String(), "options"
	}

	grubCfgContents, err := file.Read(filepath.Join(artifactsPath, grubCfgDir, isoGrubCfg))
	assert.NoError(t, err, "read grub.cfg file")
	return grubCfgContents, "linux"
}

func TestCustomizeImageLiveOSKeepKdumpFilesA(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		if *baseImageInfo.Param == "" || baseImageInfo.Version == baseImageVersionAzl2 {
			continue
		}
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSKeepKdumpFilesA(t, baseImageInfo)
		})
	}
}

func testCustomizeImageLiveOSKeepKdumpFilesA(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := *baseImageInfo.Param

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageLiveOSKeepKdumpFilesA_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// Case A0:
	//       Input: base vhdx
	//	       kdumpBootFiles=keep
	//         boot/initramfs-6.6.65.1-2.npikdump.img exists
	//         boot/vmlinuz-6.6.65.1-2.npi exists
	//       Expected: {full-os}/boot/{initramfs + kernel}
	//
	// This test case ensures we can exclude the kdump files from the /boot folder clean-up.
	//
	kdumpInitrdRelPath := "boot/initramfs-6.6.65.1-2.npikdump.img"
	kdumpVmlinuzRelPath := "boot/vmlinuz-6.6.65.1-2.npi"
	kudmpFilePaths := kdumpInitrdRelPath + ";" + kdumpVmlinuzRelPath
	configA0 := createConfig(t, baseImageInfo.Version, kudmpFilePaths, "rd.info",
		imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeKeep, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, configA0, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configA0, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)

	// Case A1:
	//       Input: iso with kdumpBootFiles=keep
	//         kdumpBootFiles=none
	//       Expected: {iso}/boot/{initramfs + kernel}
	//
	// This test case ensures that the kdump file can move from inside the full-os to the iso
	// if the user changes the kdumpBootFiles from keep to none.
	//
	kdumpBootFiles := imagecustomizerapi.KdumpBootFilesTypeNone
	configA1 := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureKdumpBootFiles,
		},
		Iso: &imagecustomizerapi.Iso{
			KdumpBootFiles: &kdumpBootFiles,
		},
	}

	err = basicCustomizeImage(t.Context(), buildDir, testDir, configA1, outImageFilePath, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	// Mount the iso
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

	// Verify the kdump files are now present on the iso
	origKdumpInitrdPath := filepath.Join(testDir, "files", kdumpInitrdRelPath)
	kdumpInitrdPath := filepath.Join(isoMountDir, kdumpInitrdRelPath)
	verifyFileContentsSame(t, origKdumpInitrdPath, kdumpInitrdPath)

	origKdumpVmlinuzPath := filepath.Join(testDir, "files", kdumpVmlinuzRelPath)
	kdumpVmlinuzPath := filepath.Join(isoMountDir, kdumpVmlinuzRelPath)
	verifyFileContentsSame(t, origKdumpVmlinuzPath, kdumpVmlinuzPath)

	// Expand initrd to a folder
	fullOSImagePath := filepath.Join(isoMountDir, "boot/initrd.img")
	fullOsDir := filepath.Join(testTempDir, "full-os")
	err = initrdutils.CreateFolderFromInitrdImage(fullOSImagePath, fullOsDir)
	if !assert.NoError(t, err) {
		return
	}
	defer os.RemoveAll(fullOsDir)

	// Verify the kdump files are not present in the full OS (no duplication)
	kdumpInitrdFullOsPath := filepath.Join(fullOsDir, kdumpInitrdRelPath)
	kdumpInitrdFullOsExists, err := file.PathExists(kdumpInitrdFullOsPath)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, kdumpInitrdFullOsExists, false, "kdump initramfs file should not exist in full-os") {
		return
	}
	kdumpVmlinuzFullOsPath := filepath.Join(fullOsDir, kdumpVmlinuzRelPath)
	kdumpVmlinuzFullOsExists, err := file.PathExists(kdumpVmlinuzFullOsPath)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, kdumpVmlinuzFullOsExists, false, "kdump vmlinuz file should not exist in full-os") {
		return
	}
}

func TestCustomizeImageLiveOSKeepKdumpFilesBC(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		if *baseImageInfo.Param == "" || baseImageInfo.Version == baseImageVersionAzl2 {
			continue
		}
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSKeepKdumpFilesBC(t, baseImageInfo)
		})
	}
}

func testCustomizeImageLiveOSKeepKdumpFilesBC(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := *baseImageInfo.Param

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageLiveOSKeepKdumpFilesBC_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// Case B:
	//       Input: base vhdx
	//	       kdumpBootFiles=keep
	//         boot/initramfs-6.6.65.1-2.npikdump.img does not exist
	//         boot/vmlinuz-6.6.65.1-2.npi exists
	//       Expected: no {full-os}/boot
	//
	// This test case ensures that if the kdump initramfs file is not present, the entire
	// /boot folder will be deleted from the full-os.
	//
	configB := createConfig(t, baseImageInfo.Version, "a.txt;boot/vmlinuz-6.6.65.1-2.npi", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeKeep, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, configB, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)

	// Case C:
	//       Input: base vhdx
	//         kdumpBootFiles=none
	//         boot/initramfs-6.6.65.1-2.npikdump.img exists
	//         boot/vmlinuz-6.6.65.1-2.npi exist
	//       Expected: no {full-os}/boot
	//
	//
	//
	configC := createConfig(t, baseImageInfo.Version, "boot/initramfs-6.6.65.1-2.npikdump.img;boot/vmlinuz-6.6.65.1-2.npi", "rd.info",
		imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err = basicCustomizeImage(t.Context(), buildDir, testDir, configC, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configC, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)
}

func TestCustomizeImageLiveOSMultiKernel(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		if *baseImageInfo.Param == "" {
			continue
		}
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSMultiKernel(t, baseImageInfo)
		})
	}
}

func testCustomizeImageLiveOSMultiKernel(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := *baseImageInfo.Param

	testTempDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageLiveOSMultiKernel_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// SELinux in Live OS is only supported with azl3
	selinuxMode := imagecustomizerapi.SELinuxModeEnforcing
	if baseImageInfo.Version == baseImageVersionAzl2 {
		selinuxMode = imagecustomizerapi.SELinuxModeDisabled
	}

	configA := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap,
		"" /*pxe url*/, true /*enlarge disk*/, true /*enable os config*/, true /*bootstrap prereqs*/, true, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, selinuxMode, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, configA, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	switch baseImageInfo.Version {
	case baseImageVersionAzl2:
		// Azl2 should fail.
		if !assert.Error(t, err) {
			return
		}
		if !assert.ErrorContains(t, err, "unsupported number of kernels") {
			return
		}
	default:
		if !assert.NoError(t, err) {
			return
		}
	}
}

// Tests:
// - raw -> iso {bootstrap} -> iso {bootstrap} -> iso {full-os}
//
// - vhdx {raw}        to ISO {bootstrap}    , with selinux enforcing + bootstrap prereqs
// - ISO  {bootstrap}  to ISO {bootstrap}    , with no OS changes
// - ISO  {bootstrap}  to ISO {full-os}      , with selinux disabled
func TestCustomizeImageLiveOSInitramfs1(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSInitramfs1Helper(t, baseImageInfo, false)
		})

		t.Run(baseImageInfo.Name+"ToolsDir", func(t *testing.T) {
			testCustomizeImageLiveOSInitramfs1Helper(t, baseImageInfo, true)
		})
	}
}

func testCustomizeImageLiveOSInitramfs1Helper(t *testing.T, baseImageInfo testBaseImageInfo, useToolsDir bool) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	toolsDir := ""
	if useToolsDir {
		toolsDir = testutils.GetDownloadedToolsDir(t, testutilsDir, baseImageInfo.Distro, baseImageInfo.Version)
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs1"+baseImageInfo.Name)
	if useToolsDir {
		testTempDir += "ToolsDir"
	}
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// SELinux in Live OS is only supported with azl3
	selinuxMode := imagecustomizerapi.SELinuxModeEnforcing
	if baseImageInfo.Version == baseImageVersionAzl2 {
		selinuxMode = imagecustomizerapi.SELinuxModeDisabled
	}

	previewFeatures := []imagecustomizerapi.PreviewFeature(nil)
	previewFeatures = append(previewFeatures, baseImageInfo.PreviewFeatures...)
	if useToolsDir {
		previewFeatures = append(previewFeatures, imagecustomizerapi.PreviewFeatureToolsDir)
	}

	configA := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, true /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, selinuxMode, useToolsDir)

	// vhdx {raw} to ISO {bootstrap}, selinux enforcing + bootstrap prereqs
	err := CustomizeImage(t.Context(), testDir, configA, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    imagecustomizerapi.ImageFormatTypeIso,
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      previewFeatures,
		SetFilesContext:      *setfilesContext,
		ToolsDir:             toolsDir,
	})
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath, baseImageInfo)

	// ISO  {bootstrap} to ISO {bootstrap}, with no OS changes
	configB := createConfig(t, baseImageInfo.Version, "b.txt", "rd.debug", imagecustomizerapi.InitramfsImageTypeBootstrap,
		"" /*pxe url*/, false /*enlarge disk*/, false /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDefault, false)

	err = CustomizeImage(t.Context(), testDir, configB, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       outImageFilePath,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    imagecustomizerapi.ImageFormatTypeIso,
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      previewFeatures,
		SetFilesContext:      *setfilesContext,
		ToolsDir:             toolsDir,
	})
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath, baseImageInfo)

	// - ISO {bootstrap} to ISO {full-os}, with selinux disabled
	configC := createConfig(t, baseImageInfo.Version, "c.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDisabled, false)

	err = CustomizeImage(t.Context(), testDir, configC, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       outImageFilePath,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    imagecustomizerapi.ImageFormatTypeIso,
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      previewFeatures,
		SetFilesContext:      *setfilesContext,
		ToolsDir:             toolsDir,
	})
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configC, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)
}

// Tests:
// - raw -> iso {full-os} -> iso {full-os} -> iso {bootstrap}
//
// - vhdx {raw}        to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {full-os}      , with selinux disabled
// - ISO  {full-os}    to ISO {bootstrap}    , with selinux enforcing + bootstrap prereqs
func TestCustomizeImageLiveOSInitramfs2(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs2")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os}, with selinux disabled
	configA := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, configA, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configA, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)

	// ISO  {full-os} to ISO {full-os}, with selinux disabled
	configB := createConfig(t, baseImageInfo.Version, "b.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, true /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err = basicCustomizeImage(t.Context(), buildDir, testDir, configB, outImageFilePath, outImageFilePath, string(imagecustomizerapi.ImageFormatTypeIso),
		baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configB, testTempDir, imagecustomizerapi.InitramfsImageTypeFullOS, outImageFilePath, baseImageInfo)

	// - ISO {full-os} to ISO {bootstrap}, with selinux enforcing

	// SELinux in Live OS is only supported with azl3
	selinuxMode := imagecustomizerapi.SELinuxModeEnforcing
	if baseImageInfo.Version == baseImageVersionAzl2 {
		selinuxMode = imagecustomizerapi.SELinuxModeDisabled
	}

	configC := createConfig(t, baseImageInfo.Version, "c.txt", "rd.shell", imagecustomizerapi.InitramfsImageTypeBootstrap,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, true /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, selinuxMode, false /*useToolsDir*/)

	err = basicCustomizeImage(t.Context(), buildDir, testDir, configC, outImageFilePath, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidateIsoContent(t, configC, testTempDir, imagecustomizerapi.InitramfsImageTypeBootstrap, outImageFilePath, baseImageInfo)
}

// Tests:
// - vhdx {raw} to ISO {full-os}, with selinux enabled -> error
func TestCustomizeImageLiveOSInitramfs3(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSInitramfs3")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	// vhdx {raw} to ISO {full-os}, with selinux disabled
	configA := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeEnforcing, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, configA, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	assert.ErrorContains(t, err, "selinux is not supported for full OS initramfs image")
}

// Tests:
// - vhdx {raw} to PXE {bootstrap}, with selinux enforcing
func TestCustomizeImageLiveOSPxe1(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("Skipping - PXE bootstrap is not supported for Azure Linux 2")
	}

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe1")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")
	pxeBootstrapUrl := "http://my-pxe-server-1/" + defaultIsoImageName

	config := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeBootstrap,
		pxeBootstrapUrl, false /*enlarge disk*/, true /*enable os config*/, true /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeEnforcing, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, config, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypePxeTar), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidatePxeContent(t, imagecustomizerapi.ImageFormatTypePxeTar, config, testTempDir, outImageFilePath, baseImageInfo)
}

// Tests:
// - vhdx {raw} to PXE {full-os}, with selinux disabled
func TestCustomizeImageLiveOSPxe2(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSPxe2")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "pxe-artifacts.tar.gz")

	config := createConfig(t, baseImageInfo.Version, "a.txt", "rd.info", imagecustomizerapi.InitramfsImageTypeFullOS,
		"" /*pxe url*/, false /*enlarge disk*/, true /*enable os config*/, false /*bootstrap prereqs*/, false, /*2 kernels*/
		imagecustomizerapi.KdumpBootFilesTypeNone, imagecustomizerapi.SELinuxModeDisabled, false /*useToolsDir*/)

	err := basicCustomizeImage(t.Context(), buildDir, testDir, config, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypePxeTar), baseImageInfo.PreviewFeatures)
	if !assert.NoError(t, err) {
		return
	}

	ValidatePxeContent(t, imagecustomizerapi.ImageFormatTypePxeTar, config, testTempDir, outImageFilePath, baseImageInfo)
}

func TestCustomizeImageLiveOSIsoNoShimEfi(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		// Azure Linux 4.0's shim package cannot be removed (it's a protected package).
		if baseImageInfo.Version == baseImageVersionAzl4 {
			continue
		}
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageLiveOSIsoNoShimEfi(t, "TestCustomizeImageLiveCdIsoNoShimEfi"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageLiveOSIsoNoShimEfi(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)
	shimPackage := "shim"

	// For arm64 and baseImageVersionAzl2, the shim package is shim-unsigned.
	if runtime.GOARCH == "arm64" && baseImageInfo.Version == baseImageVersionAzl2 {
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
	err := basicCustomizeImage(t.Context(), buildDir, testDir, config, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the boot efi file")
}

func TestCustomizeImageLiveOSIsoNoGrubEfi(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageLiveOSIsoNoGrubEfi")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, defaultIsoImageName)

	var grubPackageName string
	switch baseImageInfo.Version {
	case baseImageVersionAzl4:
		switch runtime.GOARCH {
		case "amd64":
			grubPackageName = grubEfiPackageFedoraAmd64
		default:
			grubPackageName = grubEfiPackageFedoraArm64
		}
	default:
		grubPackageName = "grub2-efi-binary"
	}

	config := &imagecustomizerapi.Config{}

	if baseImageInfo.Version == baseImageVersionAzl4 {
		// Workaround: shim-<arch> 15.8 has a hard Requires on grub2-efi-<arch>, so dnf
		// refuses to remove grub2-efi-<arch> while shim is installed. Removing shim
		// is not an option because shim is needed for the secure boot chain
		// (UEFI -> shim -> systemd-boot -> kernel) and its EFI binaries on the ESP
		// are extracted as build artifacts. Use rpm --nodeps to bypass this.
		config.Scripts = imagecustomizerapi.Scripts{
			PostCustomization: []imagecustomizerapi.Script{
				{Content: "rpm -e --nodeps " + grubPackageName},
			},
		}
	} else {
		config.OS = &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Remove: []string{grubPackageName},
			},
		}
	}

	// Customize image.
	err := basicCustomizeImage(t.Context(), buildDir, testDir, config, baseImage, outImageFilePath,
		string(imagecustomizerapi.ImageFormatTypeIso), baseImageInfo.PreviewFeatures)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to find the grub efi file")
}
