// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	osEspBootloaderDir = "/boot/efi/EFI/BOOT"
	isoBootloaderDir   = "/efi/boot"

	bootx64Binary  = "bootx64.efi"
	bootAA64Binary = "bootaa64.efi"

	grubx64Binary  = "grubx64.efi"
	grubAA64Binary = "grubaa64.efi"

	grubx64NoPrefixBinary  = "grubx64-noprefix.efi"
	grubAA64NoPrefixBinary = "grubaa64-noprefix.efi"

	systemdBootx64Binary  = "systemd-bootx64.efi"
	systemdBootAA64Binary = "systemd-bootaa64.efi"

	grubCfgDir     = "/boot/grub2"
	isoGrubCfg     = "grub.cfg"
	isoGrubCfgPath = grubCfgDir + "/" + isoGrubCfg

	pxeGrubCfg                 = "grub-pxe.cfg"
	pxeKernelsArgs             = "ip=dhcp rd.live.azldownloader=enable"
	pxeImageBaseUrlPlaceHolder = "http://pxe-image-base-url-place-holder"

	searchCommandTemplate   = "search --label %s --set root"
	rootValueLiveOSTemplate = "live:LABEL=%s"
	rootValuePxeTemplate    = "live:%s"

	isoBootDir  = "boot"
	initrdImage = "initrd.img"
	// In vhd(x)/qcow images, the kernel is named 'vmlinuz-<version>'.
	// In the ISO image, the kernel is named 'vmlinuz'.
	vmLinuzPrefix     = "vmlinuz"
	isoInitrdPath     = "/boot/" + initrdImage
	isoKernelPath     = "/boot/vmlinuz"
	isoBootloadersDir = "/efi/boot"
	isoBootImagePath  = "/boot/grub2/efiboot.img"

	// This folder is necessary to include in the initrd image so that the
	// emergency shell can work correctly with the keyboard.
	usrLibLocaleDir = "/usr/lib/locale"

	// kernel arguments template
	kernelArgsLiveOSTemplate = " rd.shell rd.live.image rd.live.dir=%s rd.live.squashimg=%s rd.live.overlay=1 rd.live.overlay.overlayfs rd.live.overlay.nouserconfirmprompt "

	liveOSDir       = "liveos"
	liveOSImage     = "rootfs.img"
	liveOSImagePath = "/" + liveOSDir + "/" + liveOSImage

	dracutConfig = `add_dracutmodules+=" dmsquash-live livenet selinux "
add_drivers+=" overlay "
hostonly="no"
`
	// the total size of a collection of files is multiplied by the
	// expansionSafetyFactor to estimate a disk size sufficient to hold those
	// files.
	expansionSafetyFactor = 1.5
)

type BootFilesArchConfig struct {
	bootBinary                  string
	grubBinary                  string
	grubNoPrefixBinary          string
	systemdBootBinary           string
	osEspBootBinaryPath         string
	osEspGrubBinaryPath         string
	osEspGrubNoPrefixBinaryPath string
	isoBootBinaryPath           string
	isoGrubBinaryPath           string
}

type IsoWorkingDirs struct {
	// 'isoBuildDir' is where intermediate files will be placed during the
	// build.
	isoBuildDir string
}

type LiveOSIsoBuilder struct {
	workingDirs IsoWorkingDirs
	artifacts   *IsoArtifactsStore
	cleanupDirs []string
}

func (b *LiveOSIsoBuilder) addCleanupDir(dirName string) {
	b.cleanupDirs = append(b.cleanupDirs, dirName)
}

func (b *LiveOSIsoBuilder) cleanUp() error {
	var err error
	for i := len(b.cleanupDirs) - 1; i >= 0; i-- {
		cleanupErr := os.RemoveAll(b.cleanupDirs[i])
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to remove (%s): %w", err, b.cleanupDirs[i], cleanupErr)
			} else {
				err = fmt.Errorf("failed to clean-up (%s): %w", b.cleanupDirs[i], cleanupErr)
			}
		}
	}
	return err
}

// populateWriteableRootfsDir
//
//	copies the contents of the rootfs partition unto the build machine.
//
// input:
//   - 'sourceDir'
//     path to full image mount root.
//   - 'writeableRootfsDir'
//     path to the folder where the contents of the rootfsDevice will be
//     copied to.
//
// output:
//   - writeableRootfsDir will hold the contents of sourceDir.
func (b *LiveOSIsoBuilder) populateWriteableRootfsDir(sourceDir, writeableRootfsDir string) error {

	logger.Log.Debugf("Creating writeable rootfs")

	err := os.MkdirAll(writeableRootfsDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder %s:\n%w", writeableRootfsDir, err)
	}

	err = copyPartitionFiles(sourceDir+"/.", writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to copy rootfs contents to a writeable folder (%s):\n%w", writeableRootfsDir, err)
	}

	return nil
}

func (b *LiveOSIsoBuilder) updateGrubCfg(isoGrubCfgFileName string, pxeGrubCfgFileName string,
	disableSELinux bool, savedConfigs *SavedConfigs, outputImageBase string) error {

	inputContentString, err := file.Read(isoGrubCfgFileName)
	if err != nil {
		return err
	}

	searchCommand := fmt.Sprintf(searchCommandTemplate, isogenerator.DefaultVolumeId)
	inputContentString, err = replaceSearchCommandAll(inputContentString, searchCommand)
	if err != nil {
		return fmt.Errorf("failed to update the search command in the iso grub.cfg:\n%w", err)
	}

	grubMkconfigEnabled := isGrubMkconfigConfig(inputContentString)
	if !grubMkconfigEnabled {
		var oldLinuxPath string
		inputContentString, oldLinuxPath, err = setLinuxPath(inputContentString, isoKernelPath)
		if err != nil {
			return fmt.Errorf("failed to update the kernel file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldLinuxPath, isoKernelPath)
		if err != nil {
			return fmt.Errorf("failed to update all the kernel file path occurances in the iso grub.cfg:\n%w", err)
		}

		var oldInitrdPath string
		inputContentString, oldInitrdPath, err = setInitrdPath(inputContentString, isoInitrdPath)
		if err != nil {
			return fmt.Errorf("failed to update the initrd file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldInitrdPath, isoInitrdPath)
		if err != nil {
			return fmt.Errorf("failed to update all the initrd file path occurances in the iso grub.cfg:\n%w", err)
		}
	} else {
		inputContentString, _, err = setLinuxOrInitrdPathAll(inputContentString, linuxCommand, isoKernelPath, true /*allowMultiple*/)
		if err != nil {
			return fmt.Errorf("failed to update the kernel file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, _, err = setLinuxOrInitrdPathAll(inputContentString, initrdCommand, isoInitrdPath, true /*allowMultiple*/)
		if err != nil {
			return fmt.Errorf("failed to update the initrd file path in the iso grub.cfg:\n%w", err)
		}
	}

	rootValue := fmt.Sprintf(rootValueLiveOSTemplate, isogenerator.DefaultVolumeId)
	inputContentString, err = replaceKernelCommandLineArgValueAll(inputContentString, "root", rootValue)
	if err != nil {
		return fmt.Errorf("failed to update the root kernel argument in the iso grub.cfg:\n%w", err)
	}

	if disableSELinux {
		inputContentString, err = updateSELinuxCommandLineHelperAll(inputContentString,
			imagecustomizerapi.SELinuxModeDisabled)
		if err != nil {
			return fmt.Errorf("failed to set SELinux mode:\n%w", err)
		}
	}

	liveosKernelArgs := fmt.Sprintf(kernelArgsLiveOSTemplate, liveOSDir, liveOSImage)
	savedArgs := GrubArgsToString(savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)
	additionalKernelCommandline := liveosKernelArgs + " " + savedArgs

	inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, additionalKernelCommandline)
	if err != nil {
		return fmt.Errorf("failed to update the kernel arguments with the LiveOS configuration and user configuration in the iso grub.cfg:\n%w", err)
	}

	err = file.Write(inputContentString, isoGrubCfgFileName)
	if err != nil {
		return fmt.Errorf("failed to write %s:\n%w", isoGrubCfgFileName, err)
	}

	// Check if the dracut version in use meets our minimum requirements for
	// PXE support.
	err = verifyDracutPXESupport(savedConfigs.OS.DracutPackageInfo)
	if err != nil {
		// MIC does not provide a way for the user to explicitly indicate that a
		// PXE bootable ISO is desired. Instead, MIC always tries to create one.
		// In cases that the source image does not meet the minimum requirements
		// for the PXE bootable ISO, MIC just reports that information to the user
		// and does not terminate the ISO creation process. No error is reported
		// because MIC does not know if the user is interested only in the ISO image,
		// or also in the PXE artifacts.
		logger.Log.Infof("cannot generate grub.cfg for PXE booting.\n%v", err)
	} else {
		err = generatePxeGrubCfg(inputContentString, savedConfigs.Pxe.IsoImageBaseUrl, savedConfigs.Pxe.IsoImageFileUrl,
			outputImageBase, pxeGrubCfgFileName)
		if err != nil {
			return fmt.Errorf("failed to create grub configuration for PXE booting.\n%w", err)
		}
	}

	return nil
}

// generatePxeGrubCfg
//
// given the content of the iso grub.cfg, this function derives the PXE
// equivalent.
//
// inputs:
//   - inputContentString:
//     iso grub.cfg content.
//   - pxeIsoImageBaseUrl:
//     url to a folder containing the iso image to download at boot time.
//     The function will append the outputImageBase to the url to form the full
//     url to the image.
//     For example, if pxeIsoImageBaseUrl is set to "http://192.168.0.1/liveos",
//     the final url will be "http://192.168.0.1/liveos/<outputImageBase>".
//     This parameter cannot be set if pxeIsoImageFileUrl is also set.
//   - pxeIsoImageFileUrl:
//     url to the iso image to download at boot time.
//     This parameter cannot be set if pxeIsoImageBaseUrl is also set.
//   - outputImageBase:
//     the generated iso name. This value will be used only if the pxeIsoImageFileUrl
//     is empty.
//   - pxeGrubCfgFileName:
//     path of file to hold the PXE grub configuration.
//
// returns:
//   - error: nil if successful, otherwise an error object.
//
// generates:
//   - grub configuration file for PXE booting.
func generatePxeGrubCfg(inputContentString string, pxeIsoImageBaseUrl string, pxeIsoImageFileUrl string,
	outputImageBase string, pxeGrubCfgFileName string) error {
	if pxeIsoImageBaseUrl != "" && pxeIsoImageFileUrl != "" {
		return fmt.Errorf("cannot set both iso image base url and full image url at the same time")
	}

	// remove 'search' commands from PXE grub.cfg because it is not needed.
	inputContentString, err := removeCommandAll(inputContentString, "search")
	if err != nil {
		return fmt.Errorf("failed to remove the 'search' commands from PXE grub.cfg:\n%w", err)
	}

	// If the specified URL is not a full path to an iso, append the generated
	// iso file name to it.
	if pxeIsoImageFileUrl == "" {
		pxeIsoImageFileUrl, err = url.JoinPath(pxeIsoImageBaseUrl, outputImageBase)
		if err != nil {
			return fmt.Errorf("failed to concatenate URL (%s) and (%s)\n%w", pxeIsoImageBaseUrl, outputImageBase, err)
		}
	}
	rootValue := fmt.Sprintf(rootValuePxeTemplate, pxeIsoImageFileUrl)
	inputContentString, err = replaceKernelCommandLineArgValueAll(inputContentString, "root", rootValue)
	if err != nil {
		return fmt.Errorf("failed to update the root kernel argument with the PXE iso image url in the PXE grub.cfg:\n%w", err)
	}

	inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, pxeKernelsArgs)
	if err != nil {
		return fmt.Errorf("failed to append the kernel arguments (%s) in the PXE grub.cfg:\n%w", pxeKernelsArgs, err)
	}

	err = file.Write(inputContentString, pxeGrubCfgFileName)
	if err != nil {
		return fmt.Errorf("failed to write %s:\n%w", pxeGrubCfgFileName, err)
	}

	return nil
}

// prepareArtifactsFromFullImage
//
//	extracts and generates all LiveOS Iso artifacts from a given raw full disk
//	image (has boot and rootfs partitions).
//
// inputs:
//   - 'inputArtifactsStore'
//     If input is an ISO image, this holds its artifacts.
//   - 'rawImageFile':
//     path to an existing raw full disk image (i.e. image with boot
//     partition and a rootfs partition).
//   - 'requestedSelinuxMode'
//     requested selinux mode by the user (from os.selinux.mode).
//   - 'extraCommandLine':
//     extra kernel command line arguments to add to grub.
//   - 'pxeIsoImageBaseUrl':
//     url to the folder holding the iso to download at boot time.
//     Cannot be specified if pxeIsoImageFileUrl is specified.
//   - 'pxeIsoImageFileUrl':
//     url to the iso image to download at boot time.
//     Cannot be specified if pxeIsoImageBaseUrl is specified.
//   - 'outputImageBase':
//     output image iso name.
//
// outputs:
//   - all the extracted/generated artifacts will be placed in the
//     `LiveOSIsoBuilder.artifacts.files.artifactsDir` folder.
//   - the paths to individual artifacts are found in the
//     `LiveOSIsoBuilder.artifacts` data structure.
func (b *LiveOSIsoBuilder) prepareArtifactsFromFullImage(inputArtifactsStore *IsoArtifactsStore, rawImageFile string, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
	extraCommandLine []string, pxeIsoImageBaseUrl string, pxeIsoImageFileUrl string, outputImageBase string) error {
	logger.Log.Infof("Preparing iso artifacts")

	logger.Log.Debugf("Connecting to raw image (%s)", rawImageFile)
	rawImageConnection, _, err := connectToExistingImage(rawImageFile, b.workingDirs.isoBuildDir, "readonly-rootfs-mount", false /*includeDefaultMounts*/)
	if err != nil {
		return err
	}
	defer rawImageConnection.Close()

	writeableRootfsDir := filepath.Join(b.workingDirs.isoBuildDir, "writeable-rootfs")
	err = b.populateWriteableRootfsDir(rawImageConnection.Chroot().RootDir(), writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to copy the contents of rootfs from image (%s) to local folder (%s):\n%w", rawImageFile, writeableRootfsDir, err)
	}

	storeFolder := filepath.Join(b.workingDirs.isoBuildDir, "from-iso-and-raw")
	artifacts, err := createIsoArtifactStoreFromMountedImage(inputArtifactsStore, writeableRootfsDir, storeFolder)
	if err != nil {
		return err
	}
	b.artifacts = artifacts

	// Combine the current configuration with the saved configuration
	updatedSavedConfigs, err := updateSavedConfigs(b.artifacts.files.savedConfigsFilePath, extraCommandLine, pxeIsoImageBaseUrl,
		pxeIsoImageFileUrl, b.artifacts.info.dracutPackageInfo, requestedSelinuxMode, b.artifacts.info.selinuxPolicyPackageInfo)
	if err != nil {
		return fmt.Errorf("failed to combine saved configurations with new configuration:\n%w", err)
	}

	// Figure out the selinux situation
	// Note that by now, the user selinux config has been applied to the image,
	// so checking only 'imageSELinuxMode' is sufficient to determine whether
	// selinux is enabled or not for this image (regardless of the source of
	// that configuration).
	disableSELinux := false
	if b.artifacts.info.seLinuxMode != imagecustomizerapi.SELinuxModeDisabled {
		// SELinux is enabled (either in the base image, or requested by the user)
		err = verifyNoLiveOsSelinuxBlockers(updatedSavedConfigs.OS.DracutPackageInfo, updatedSavedConfigs.OS.SELinuxPolicyPackageInfo)
		if err != nil {
			// We need to determine whether the source of enablment is user
			// explicit configuration or the base image.
			if updatedSavedConfigs.OS.RequestedSELinuxMode != imagecustomizerapi.SELinuxModeDisabled &&
				updatedSavedConfigs.OS.RequestedSELinuxMode != imagecustomizerapi.SELinuxModeDefault {
				return fmt.Errorf("SELinux cannot be enabled due to older dracut and selinux-policy package versions:\n%w", err)
			} else {
				logger.Log.Infof("SELinux disabled due to older dracut and selinux-policy package versions:\n%s", err)
			}

			disableSELinux = true
		}
	}

	// Update grug.cfg
	err = b.updateGrubCfg(b.artifacts.files.isoGrubCfgPath, b.artifacts.files.pxeGrubCfgPath, disableSELinux,
		updatedSavedConfigs, outputImageBase)
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the ISO bootimage (/boot/grub2/efiboot.img)
	b.artifacts.files.isoBootImagePath = filepath.Join(b.artifacts.files.artifactsDir, isoBootImagePath)
	err = isogenerator.BuildIsoBootImage(b.workingDirs.isoBuildDir, b.artifacts.files.bootEfiPath,
		b.artifacts.files.grubEfiPath, b.artifacts.files.isoBootImagePath)
	if err != nil {
		return fmt.Errorf("failed to build iso boot image:\n%w", err)
	}

	// Generate the initrd image
	outputInitrdPath := filepath.Join(b.artifacts.files.artifactsDir, initrdImage)
	err = createInitrdImage(writeableRootfsDir, b.artifacts.info.kernelVersion, outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to create initrd image:\n%w", err)
	}
	b.artifacts.files.initrdImagePath = outputInitrdPath

	// Generate the squashfs image
	outputSquashfsPath := filepath.Join(b.artifacts.files.artifactsDir, liveOSImage)
	err = createSquashfsImage(writeableRootfsDir, outputSquashfsPath)
	if err != nil {
		return fmt.Errorf("failed to create squashfs image:\n%w", err)
	}
	b.artifacts.files.squashfsImagePath = outputSquashfsPath

	return nil
}

// createLiveOSIsoImage
//
//	main function to create a LiveOS ISO image from a raw full disk image file.
//
// inputs:
//
//   - 'buildDir':
//     path build directory (can be shared with other tools).
//   - 'baseConfigPath'
//     path to the folder where the mic configuration was loaded from.
//     This path will be used to construct absolute paths for file references
//     defined in the config.
//   - 'inputIsoBuilder'
//     an optional LiveOSIsoBuilder that holds the state of the original input
//     iso if one was provided. If present, this function will copy all files
//     from the inputIsoBuilder.artifacts.additionalFiles to the new iso
//     if the destination is not already defined (for the new iso).
//     This is used to carry over any files from a previously customized iso
//     to the new one.
//   - 'isoConfig'
//     user provided configuration for the iso image.
//   - 'pxeConfig'
//     user provided configuration for the PXE flow.
//   - 'rawImageFile':
//     path to an existing raw full disk image (has boot + rootfs partitions).
//   - 'outputImagePath':
//     path to the output image.
//   - 'outputPXEArtifactsDir'
//     optional directory path where the PXE artifacts will be exported to if
//     specified.
//
// outputs:
//
//	creates a LiveOS ISO image.
func createLiveOSIsoImage(buildDir, baseConfigPath string, inputIsoBuilder *LiveOSIsoBuilder, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
	isoConfig *imagecustomizerapi.Iso, pxeConfig *imagecustomizerapi.Pxe, rawImageFile, outputImagePath string,
	outputPXEArtifactsDir string) (err error) {

	var extraCommandLine []string
	var additionalIsoFiles imagecustomizerapi.AdditionalFileList
	if isoConfig != nil {
		extraCommandLine = isoConfig.KernelCommandLine.ExtraCommandLine
		additionalIsoFiles = isoConfig.AdditionalFiles
	}

	pxeIsoImageBaseUrl := ""
	if pxeConfig != nil {
		pxeIsoImageBaseUrl = pxeConfig.IsoImageBaseUrl
	}

	pxeIsoImageFileUrl := ""
	if pxeConfig != nil {
		pxeIsoImageFileUrl = pxeConfig.IsoImageFileUrl
	}

	isoBuildDir := filepath.Join(buildDir, "liveosbuild")

	isoBuilder := &LiveOSIsoBuilder{
		//
		// buildDir (might be shared with other build tools)
		//  |--tmp   (LiveOSIsoBuilder specific)
		//     |--<various mount points>
		//     |--artifacts        (extracted and generated artifacts)
		//
		workingDirs: IsoWorkingDirs{
			isoBuildDir: isoBuildDir,
		},
	}
	defer func() {
		cleanupErr := os.RemoveAll(isoBuilder.workingDirs.isoBuildDir)
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to clean-up (%s): %w", err, isoBuilder.workingDirs.isoBuildDir, cleanupErr)
			} else {
				err = fmt.Errorf("failed to clean-up (%s): %w", isoBuilder.workingDirs.isoBuildDir, cleanupErr)
			}
		}
	}()

	err = isoBuilder.prepareArtifactsFromFullImage(inputIsoBuilder.artifacts,
		rawImageFile, requestedSelinuxMode, extraCommandLine,
		pxeIsoImageBaseUrl, pxeIsoImageFileUrl, filepath.Base(outputImagePath))
	if err != nil {
		return err
	}

	err = isoBuilder.createIsoImageAndPXEFolder(baseConfigPath, additionalIsoFiles, outputImagePath, outputPXEArtifactsDir)
	if err != nil {
		return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
	}

	return nil
}

// createIsoBuilderFromIsoImage
//
//   - given an iso image, this function extracts its contents, scans them, and
//     constructs a LiveOSIsoBuilder object filling out as many of its fields as
//     possible.
//
// inputs:
//
//   - 'buildDir':
//     path build directory (can be shared with other tools).
//   - 'buildDirAbs'
//     the absolute path of 'buildDir'.
//   - 'isoImageFile'
//     the source iso image file to extract/scan.
//
// outputs:
//
//   - returns an instance of LiveOSIsoBuilder populated with all the paths of the
//     extracted contents.
func createIsoBuilderFromIsoImage(buildDir string, isoImageFile string) (isoBuilder *LiveOSIsoBuilder, err error) {

	isoBuildDir := filepath.Join(buildDir, "liveosbuild")

	isoBuilder = &LiveOSIsoBuilder{
		//
		// buildDir (might be shared with other build tools)
		//  |--tmp   (LiveOSIsoBuilder specific)
		//     |--<various mount points>
		//     |--artifacts        (extracted and generated artifacts)
		//
		workingDirs: IsoWorkingDirs{
			isoBuildDir: isoBuildDir,
		},
	}
	defer func() {
		if err != nil {
			cleanupErr := isoBuilder.cleanUp()
			if cleanupErr != nil {
				err = fmt.Errorf("%w:\nfailed to clean-up:\n%w", err, cleanupErr)
			}
		}
	}()

	// create iso build folder
	err = os.MkdirAll(isoBuildDir, os.ModePerm)
	if err != nil {
		return isoBuilder, fmt.Errorf("failed to create folder %s:\n%w", isoBuildDir, err)
	}
	isoBuilder.addCleanupDir(isoBuildDir)

	storeFolder := filepath.Join(isoBuildDir, "from-iso")
	artifacts, err := createIsoArtifactStoreFromIsoImage(isoImageFile, storeFolder)
	if err != nil {
		return isoBuilder, fmt.Errorf("failed to create artifacts store from (%s):\n%w", isoImageFile, err)
	}
	isoBuilder.artifacts = artifacts

	return isoBuilder, nil
}

// createImageFromUnchangedOS
//
//   - assuming the LiveOSIsoBuilder instance has all its artifacts populated,
//     this function goes straight to updating grub and re-packaging the
//     artifacts into an iso image. It does not re-create the initrd.img or
//     the squashfs.img. This speeds-up customizing iso images when there are
//     no customizations applicable to the OS (i.e. to the squashfs.img).
//
// inputs:
//
//   - 'baseConfigPath':
//     path to where the configuration is loaded from. This is used to resolve
//     relative paths.
//   - 'isoConfig'
//     user provided configuration for the iso image.
//   - 'pxeConfig'
//     user provided configuration for the PXE flow.
//   - 'outputImagePath':
//     path to a the output image.
//   - 'outputPXEArtifactsDir'
//     optional directory path where the PXE artifacts will be exported to if
//     specified.
//
// outputs:
//
//   - creates an iso image.
func (b *LiveOSIsoBuilder) createImageFromUnchangedOS(baseConfigPath string, isoConfig *imagecustomizerapi.Iso,
	pxeConfig *imagecustomizerapi.Pxe, outputImagePath string, outputPXEArtifactsDir string) error {

	logger.Log.Infof("Creating LiveOS iso image using unchanged OS partitions")

	var extraCommandLine []string
	var additionalIsoFiles imagecustomizerapi.AdditionalFileList
	if isoConfig != nil {
		extraCommandLine = isoConfig.KernelCommandLine.ExtraCommandLine
		additionalIsoFiles = isoConfig.AdditionalFiles
	}

	pxeIsoImageBaseUrl := ""
	if pxeConfig != nil {
		pxeIsoImageBaseUrl = pxeConfig.IsoImageBaseUrl
	}

	pxeIsoImageFileUrl := ""
	if pxeConfig != nil {
		pxeIsoImageFileUrl = pxeConfig.IsoImageFileUrl
	}

	// Note that in this ISO build flow, there is no os configuration, and hence
	// no selinux configuration. So, we will set it to default (i.e. unspecified)
	// and let any saved data override if present.
	requestedSelinuxMode := imagecustomizerapi.SELinuxModeDefault

	updatedSavedConfigs, err := updateSavedConfigs(b.artifacts.files.savedConfigsFilePath, extraCommandLine, pxeIsoImageBaseUrl,
		pxeIsoImageFileUrl, nil /*dracut pkg info*/, requestedSelinuxMode, nil /*selinux policy pkg info*/)
	if err != nil {
		return fmt.Errorf("failed to combine saved configurations with new configuration:\n%w", err)
	}

	// SELinux cannot be enabled/disabled in this flow since, by definition,
	// the config os.selinux is not present. As a result, we will just keep
	// SELinux configuration unchanged.
	// Setting disableSELinux to false tells updateGrubCfg to, well, not disable
	// selinux and not enable it either.
	disableSELinux := false

	err = b.updateGrubCfg(b.artifacts.files.isoGrubCfgPath, b.artifacts.files.pxeGrubCfgPath, disableSELinux, updatedSavedConfigs, filepath.Base(outputImagePath))
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	err = b.createIsoImageAndPXEFolder(baseConfigPath, additionalIsoFiles, outputImagePath, outputPXEArtifactsDir)
	if err != nil {
		return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
	}

	return nil
}

// createIsoImageAndPXEFolder
//
//   - This function create the liveos iso image and also populates the PXE
//     artifacts folder.
//
// inputs:
//
//   - additionalIsoFiles:
//     map of addition files to copy to the iso media.
//     sourcePath -> [ targetPath0, targetPath1, ...]
//   - outputIsoImage:
//     path to the iso image to be created upon successful copmletion of this
//     function.
//   - 'outputPXEArtifactsDir'
//     path to the output directory where the extract artifacts will be saved to.
//
// outputs:
//
//   - create an iso image.
//   - creates a folder with PXE artifacts.
func (b *LiveOSIsoBuilder) createIsoImageAndPXEFolder(baseConfigPath string, additionalIsoFiles imagecustomizerapi.AdditionalFileList, outputImagePath string,
	outputPXEArtifactsDir string) error {

	dumpArtifactsStore(b.artifacts, "used to create final iso")

	err := createIsoImage(b.workingDirs.isoBuildDir, b.artifacts.files, baseConfigPath, additionalIsoFiles, outputImagePath)
	if err != nil {
		return fmt.Errorf("failed to create the Iso image.\n%w", err)
	}

	if outputPXEArtifactsDir != "" {
		err = verifyDracutPXESupport(b.artifacts.info.dracutPackageInfo)
		if err != nil {
			return fmt.Errorf("failed to verify Dracut's PXE support.\n%w", err)
		}
		err = populatePXEArtifactsDir(outputImagePath, b.workingDirs.isoBuildDir, outputPXEArtifactsDir)
		if err != nil {
			return fmt.Errorf("failed to populate the PXE artifacts folder.\n%w", err)
		}
	}

	return nil
}

// populatePXEArtifactsDir
//
//   - This function takes in an liveos iso, and extracts its artifacts unto a
//     folder for easier copying to a PXE server later by the user.
//   - It also renames the liveos iso grub-pxe.cfg to grub.cfg.
//
// inputs:
//
//   - 'isoImagePath':
//     path to a liveos iso image.
//   - 'buildDir'
//     path to a directory to hold intermediate files.
//   - 'outputPXEArtifactsDir'
//     path to the output directory where the extract artifacts will be saved to.
//
// outputs:
//
//   - creates a folder with PXE artifacts.
func populatePXEArtifactsDir(isoImagePath string, buildDir string, outputPXEArtifactsDir string) error {

	logger.Log.Infof("Copying PXE artifacts to (%s)", outputPXEArtifactsDir)

	// Extract all files from the iso image file.
	err := extractIsoImageContents(buildDir, isoImagePath, outputPXEArtifactsDir)
	if err != nil {
		return err
	}

	// Replace the iso grub.cfg with the PXE grub.cfg
	isoGrubCfgPath := filepath.Join(outputPXEArtifactsDir, grubCfgDir, isoGrubCfg)
	pxeGrubCfgPath := filepath.Join(outputPXEArtifactsDir, grubCfgDir, pxeGrubCfg)
	err = file.Copy(pxeGrubCfgPath, isoGrubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to copy (%s) to (%s) while populating the PXE artifacts directory:\n%w", pxeGrubCfgPath, isoGrubCfgPath, err)
	}

	err = os.RemoveAll(pxeGrubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to remove file (%s):\n%w", pxeGrubCfgPath, err)
	}

	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}
	// Move bootloader files from under '<pxe-folder>/efi/boot' to '<pxe-folder>/'
	bootloaderSrcDir := filepath.Join(outputPXEArtifactsDir, isoBootloadersDir)
	bootloaderFiles := []string{bootFilesConfig.bootBinary, bootFilesConfig.grubBinary}

	for _, bootloaderFile := range bootloaderFiles {
		sourcePath := filepath.Join(bootloaderSrcDir, bootloaderFile)
		targetPath := filepath.Join(outputPXEArtifactsDir, bootloaderFile)
		err = file.Move(sourcePath, targetPath)
		if err != nil {
			return fmt.Errorf("failed to move boot loader file from (%s) to (%s) while generated the PXE artifacts folder:\n%w", sourcePath, targetPath, err)
		}
	}

	// Remove the empty 'pxe-folder>/efi' folder.
	isoEFIDir := filepath.Join(outputPXEArtifactsDir, "efi")
	err = os.RemoveAll(isoEFIDir)
	if err != nil {
		return fmt.Errorf("failed to remove folder (%s):\n%w", isoEFIDir, err)
	}

	// The iso image file itself must be placed in the PXE folder because
	// dracut livenet module will download it.
	artifactsIsoImagePath := filepath.Join(outputPXEArtifactsDir, filepath.Base(isoImagePath))
	err = file.Copy(isoImagePath, artifactsIsoImagePath)
	if err != nil {
		return fmt.Errorf("failed to copy (%s) while populating the PXE artifacts directory:\n%w", isoImagePath, err)
	}

	return nil
}
