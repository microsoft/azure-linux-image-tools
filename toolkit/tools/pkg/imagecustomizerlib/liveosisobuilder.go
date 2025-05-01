// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
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

// prepareLiveOSDir
//
//	given a rootfs, this function:
//	- extracts the kernel version, and the files under the boot folder.
//	- stages bootloaders and vmlinuz to a specific folder structure.
//	-prepares the rootfs to run dracut (dracut will generate the initrd later).
//	- creates the squashfs.
//
// inputs:
//   - 'inputSavedConfigsFilePath':
//   - writeableRootfsDir:
//     A writeable folder where the rootfs content is.
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
// outputs
//   - customized writeableRootfsDir (new files, deleted files, etc)
//   - extracted artifacts
func (b *LiveOSIsoBuilder) prepareLiveOSDir(inputSavedConfigsFilePath string, writeableRootfsDir string,
	requestedSelinuxMode imagecustomizerapi.SELinuxMode, extraCommandLine []string,
	pxeIsoImageBaseUrl string, pxeIsoImageFileUrl string, outputImageBase string) error {

	artifacts, err := createIsoArtifactStoreFromMountedImage(b.workingDirs.isoBuildDir, writeableRootfsDir)
	if err != nil {
		return err
	}
	b.artifacts = artifacts

	exists, err := file.PathExists(inputSavedConfigsFilePath)
	if err != nil {
		return err
	}
	if exists {
		err = file.Copy(inputSavedConfigsFilePath, b.artifacts.files.savedConfigsFilePath)
		if err != nil {
			return fmt.Errorf("failed to saved arguments file:\n%w", err)
		}
	}

	// Combine the current state
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

	err = b.updateGrubCfg(b.artifacts.files.isoGrubCfgPath, b.artifacts.files.pxeGrubCfgPath, disableSELinux, updatedSavedConfigs, outputImageBase)
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	b.artifacts.files.isoBootImagePath = filepath.Join(b.artifacts.files.artifactsDir, isoBootImagePath)
	err = isogenerator.BuildIsoBootImage(b.workingDirs.isoBuildDir, b.artifacts.files.bootEfiPath, b.artifacts.files.grubEfiPath, b.artifacts.files.isoBootImagePath)
	if err != nil {
		return fmt.Errorf("failed to build iso boot image:\n%w", err)
	}

	return nil
}

// prepareArtifactsFromFullImage
//
//	extracts and generates all LiveOS Iso artifacts from a given raw full disk
//	image (has boot and rootfs partitions).
//
// inputs:
//   - 'inputSavedConfigsFilePath':
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
func (b *LiveOSIsoBuilder) prepareArtifactsFromFullImage(inputSavedConfigsFilePath string, rawImageFile string, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
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

	err = b.prepareLiveOSDir(inputSavedConfigsFilePath, writeableRootfsDir, requestedSelinuxMode, extraCommandLine,
		pxeIsoImageBaseUrl, pxeIsoImageFileUrl, outputImageBase)
	if err != nil {
		return fmt.Errorf("failed to convert rootfs folder to a LiveOS folder:\n%w", err)
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
//   - 'inputIsoArtifacts'
//     an optional LiveOSIsoBuilder that holds the state of the original input
//     iso if one was provided. If present, this function will copy all files
//     from the inputIsoArtifacts.artifacts.additionalFiles to the new iso
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
func createLiveOSIsoImage(buildDir, baseConfigPath string, inputIsoArtifacts *LiveOSIsoBuilder, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
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
	// isoArtifactsDir := filepath.Join(isoBuildDir, "artifacts")

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

	// if there is an input iso, make sure to pick-up its saved kernel args
	// file.
	inputSavedConfigsFilePath := ""
	if inputIsoArtifacts != nil {
		inputSavedConfigsFilePath = inputIsoArtifacts.artifacts.files.savedConfigsFilePath
	}

	err = isoBuilder.prepareArtifactsFromFullImage(inputSavedConfigsFilePath, rawImageFile, requestedSelinuxMode, extraCommandLine,
		pxeIsoImageBaseUrl, pxeIsoImageFileUrl, filepath.Base(outputImagePath))
	if err != nil {
		return err
	}

	// If we started from an input iso (not an input vhd(x)/qcow), then there
	// might be additional files that are not defined in the current user
	// configuration. Below, we loop through the files we have captured so far
	// and append any file that was in the input iso and is not included
	// already. This also ensures that no file from the input iso overwrites
	// a newer version that has just been created.
	if inputIsoArtifacts != nil {
		for inputSourceFile, inputTargetFile := range inputIsoArtifacts.artifacts.files.additionalFiles {
			found := false
			for _, targetFile := range isoBuilder.artifacts.files.additionalFiles {
				if inputTargetFile == targetFile {
					found = true
					break
				}
			}

			if !found {
				isoBuilder.artifacts.files.additionalFiles[inputSourceFile] = inputTargetFile
			}
		}
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
func createIsoBuilderFromIsoImage(buildDir string, buildDirAbs string, isoImageFile string) (isoBuilder *LiveOSIsoBuilder, err error) {

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

	artifacts, err := createIsoArtifactStoreFromIsoImage(buildDirAbs, isoImageFile)
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

// getSizeOnDiskInBytes
//
//   - given a folder, it calculates the total size in bytes of its contents.
//
// inputs:
//
//   - 'fileOrDir':
//     root folder to calculate its size.
//
// outputs:
//
//   - returns the size in bytes.
func getSizeOnDiskInBytes(fileOrDir string) (size uint64, err error) {
	logger.Log.Debugf("Calculating total size for (%s)", fileOrDir)

	duStdout, _, err := shell.Execute("du", "-s", fileOrDir)
	if err != nil {
		return 0, fmt.Errorf("failed to find the size of the specified file/dir using 'du' for (%s):\n%w", fileOrDir, err)
	}

	// parse and get count and unit
	diskSizeRegex := regexp.MustCompile(`^(\d+)\s+`)
	matches := diskSizeRegex.FindStringSubmatch(duStdout)
	if matches == nil || len(matches) < 2 {
		return 0, fmt.Errorf("failed to parse 'du -s' output (%s).", duStdout)
	}

	sizeInKbsString := matches[1]
	sizeInKbs, err := strconv.ParseUint(sizeInKbsString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse disk size (%d):\n%w", sizeInKbs, err)
	}

	return sizeInKbs * diskutils.KiB, nil
}

// getDiskSizeEstimateInMBs
//
//   - given a folder, it calculates the size of a disk image that can hold
//     all of its contents.
//   - The amount of disk space a file occupies depends on the block size of the
//     host file system. If many files are smaller than a block size, there will
//     be a lot of waste. If files are very large, there will be very little
//     waste. It is hard to predict how much disk space a set of a files will
//     occupy without enumerating the sizes of all the files and knowing the
//     target block size. In this function, we use an optimistic approach which
//     calculates the required disk space by multiplying the total file size by
//     a safety factor - i.e. safe that it will be able t hold all the contents.
//
// inputs:
//
//   - 'filesOrDirs':
//     list of files or directories to include in the calculation.
//   - 'safetyFactor':
//     a multiplier used with the total number of bytes calculated.
//
// outputs:
//
//   - returns the size in mega bytes.
func getDiskSizeEstimateInMBs(filesOrDirs []string, safetyFactor float64) (size uint64, err error) {

	totalSizeInBytes := uint64(0)
	for _, fileOrDir := range filesOrDirs {
		sizeInBytes, err := getSizeOnDiskInBytes(fileOrDir)
		if err != nil {
			return 0, fmt.Errorf("failed to get the size of (%s) on disk while estimating total disk size:\n%w", fileOrDir, err)
		}
		totalSizeInBytes += sizeInBytes
	}

	sizeInMBs := totalSizeInBytes/diskutils.MiB + 1
	estimatedSizeInMBs := uint64(float64(sizeInMBs) * safetyFactor)
	return estimatedSizeInMBs, nil
}

// createWriteableImageFromArtifacts
//
//   - given a squashfs image file, it creates a writeable image with two
//     partitions, and copies the contents of the squashfs unto that writeable
//     image.
//   - the squashfs image file must be extracted from a previously created
//     LiveOS iso and is specified by the LiveOSIsoBuilder.artifacts.squashfsImagePath.
//
// inputs:
//
//   - 'buildDir':
//     path build directory (can be shared with other tools).
//   - 'rawImageFile':
//     the name of the raw image to create and populate with the contents of
//     the squashfs.
//
// outputs:
//
//   - creates the specified writeable image.
func (b *LiveOSIsoBuilder) createWriteableImageFromArtifacts(buildDir, rawImageFile string) error {

	logger.Log.Infof("Creating writeable image from squashfs (%s)", b.artifacts.files.squashfsImagePath)

	// rootfs folder (mount squash fs)
	squashMountDir, err := os.MkdirTemp(buildDir, "tmp-squashfs-mount-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
	}
	defer os.RemoveAll(squashMountDir)

	squashfsLoopDevice, err := safeloopback.NewLoopback(b.artifacts.files.squashfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to create loop device for (%s):\n%w", b.artifacts.files.squashfsImagePath, err)
	}
	defer squashfsLoopDevice.Close()

	squashfsMount, err := safemount.NewMount(squashfsLoopDevice.DevicePath(), squashMountDir,
		"squashfs" /*fstype*/, 0 /*flags*/, "" /*data*/, false /*makeAndDelete*/)
	if err != nil {
		return err
	}
	defer squashfsMount.Close()

	// boot folder (from artifacts)
	artifactsBootDir := filepath.Join(b.artifacts.files.artifactsDir, "boot")

	imageContentList := []string{
		squashMountDir,
		b.artifacts.files.bootEfiPath,
		b.artifacts.files.grubEfiPath,
		artifactsBootDir}

	// estimate the new disk size
	safeDiskSizeMB, err := getDiskSizeEstimateInMBs(imageContentList, expansionSafetyFactor)
	if err != nil {
		return fmt.Errorf("failed to calculate the disk size of %s:\n%w", squashMountDir, err)
	}

	logger.Log.Debugf("safeDiskSizeMB = %d", safeDiskSizeMB)

	// define a disk layout with a boot partition and a rootfs partition
	maxDiskSizeMB := imagecustomizerapi.DiskSize(safeDiskSizeMB * diskutils.MiB)
	bootPartitionStart := imagecustomizerapi.DiskSize(1 * diskutils.MiB)
	bootPartitionEnd := imagecustomizerapi.DiskSize(9 * diskutils.MiB)

	diskConfig := imagecustomizerapi.Disk{
		PartitionTableType: imagecustomizerapi.PartitionTableTypeGpt,
		MaxSize:            &maxDiskSizeMB,
		Partitions: []imagecustomizerapi.Partition{
			{
				Id:    "esp",
				Start: &bootPartitionStart,
				End:   &bootPartitionEnd,
				Type:  imagecustomizerapi.PartitionTypeESP,
			},
			{
				Id:    "rootfs",
				Start: &bootPartitionEnd,
			},
		},
	}

	fileSystemConfigs := []imagecustomizerapi.FileSystem{
		{
			DeviceId:    "esp",
			PartitionId: "esp",
			Type:        imagecustomizerapi.FileSystemTypeFat32,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path:    "/boot/efi",
				Options: "umask=0077",
			},
		},
		{
			DeviceId:    "rootfs",
			PartitionId: "rootfs",
			Type:        imagecustomizerapi.FileSystemTypeExt4,
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
	}

	targetOs, err := targetos.GetInstalledTargetOs(squashMountDir)
	if err != nil {
		return fmt.Errorf("failed to determine target OS of ISO squashfs:\n%w", err)
	}

	// populate the newly created disk image with content from the squash fs
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		// At the point when this copy will be executed, both the boot and the
		// root partitions will be mounted, and the files of /boot/efi will
		// land on the the boot partition, while the rest will be on the rootfs
		// partition.
		err := copyPartitionFiles(squashMountDir+"/.", imageChroot.RootDir())
		if err != nil {
			return fmt.Errorf("failed to copy squashfs contents to a writeable disk:\n%w", err)
		}

		// Note that before the LiveOS ISO is first created, the boot folder is
		// removed from the squashfs since it is not needed. The boot artifacts
		// are stored directly on the ISO media outside the squashfs image.
		// Now that we are re-constructing the full file system, we need to
		// pull the boot artifacts back into the full file system so that
		// it is restored to its original state and subsequent customization
		// or extraction can proceed transparently.

		err = copyPartitionFiles(artifactsBootDir, imageChroot.RootDir())
		if err != nil {
			return fmt.Errorf("failed to copy (%s) contents to a writeable disk:\n%w", artifactsBootDir, err)
		}

		targetEfiDir := filepath.Join(imageChroot.RootDir(), "boot/efi/EFI/BOOT")
		err = os.MkdirAll(targetEfiDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create destination efi directory (%s):\n%w", targetEfiDir, err)
		}

		targetShimPath := filepath.Join(targetEfiDir, filepath.Base(b.artifacts.files.bootEfiPath))
		err = file.Copy(b.artifacts.files.bootEfiPath, targetShimPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", b.artifacts.files.bootEfiPath, targetShimPath, err)
		}

		targetGrubPath := filepath.Join(targetEfiDir, filepath.Base(b.artifacts.files.grubEfiPath))
		err = file.Copy(b.artifacts.files.grubEfiPath, targetGrubPath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) to (%s):\n%w", b.artifacts.files.grubEfiPath, targetGrubPath, err)
		}

		return err
	}

	// create the new raw disk image
	writeableChrootDir := "writeable-raw-image"
	_, err = createNewImage(targetOs, rawImageFile, diskConfig, fileSystemConfigs, buildDir, writeableChrootDir,
		installOSFunc)
	if err != nil {
		return fmt.Errorf("failed to copy squashfs into new writeable image (%s):\n%w", rawImageFile, err)
	}

	err = squashfsMount.CleanClose()
	if err != nil {
		return err
	}

	err = squashfsLoopDevice.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
