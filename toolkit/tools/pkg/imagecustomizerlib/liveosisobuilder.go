// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

func populateWriteableRootfsDir(sourceDir, writeableRootfsDir string) error {

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

func createLiveOSIsoImage(buildDir, baseConfigPath string, inputArtifactsStore *IsoArtifactsStore, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
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
	defer func() {
		cleanupErr := os.RemoveAll(isoBuildDir)
		if cleanupErr != nil {
			if err != nil {
				err = fmt.Errorf("%w:\nfailed to clean-up (%s): %w", err, isoBuildDir, cleanupErr)
			} else {
				err = fmt.Errorf("failed to clean-up (%s): %w", isoBuildDir, cleanupErr)
			}
		}
	}()

	logger.Log.Debugf("Connecting to raw image (%s)", rawImageFile)
	rawImageConnection, _, _, _, err := connectToExistingImage(rawImageFile, isoBuildDir, "readonly-rootfs-mount", false /*includeDefaultMounts*/)
	if err != nil {
		return err
	}
	defer rawImageConnection.Close()

	// From raw image to a writeable folder
	writeableRootfsDir := filepath.Join(isoBuildDir, "writeable-rootfs")
	err = populateWriteableRootfsDir(rawImageConnection.Chroot().RootDir(), writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to copy the contents of rootfs from image (%s) to local folder (%s):\n%w", rawImageFile, writeableRootfsDir, err)
	}

	// Create the ISO artifacts store
	storeFolder := filepath.Join(isoBuildDir, "from-iso-and-raw")
	artifactsStore, err := createIsoArtifactStoreFromMountedImage(inputArtifactsStore, writeableRootfsDir, storeFolder)
	if err != nil {
		return err
	}

	// Combine the current configuration with the saved configuration
	updatedSavedConfigs, err := updateSavedConfigs(artifactsStore.files.savedConfigsFilePath, extraCommandLine, pxeIsoImageBaseUrl,
		pxeIsoImageFileUrl, artifactsStore.info.dracutPackageInfo, requestedSelinuxMode, artifactsStore.info.selinuxPolicyPackageInfo)
	if err != nil {
		return fmt.Errorf("failed to combine saved configurations with new configuration:\n%w", err)
	}

	// Figure out the selinux situation
	// Note that by now, the user selinux config has been applied to the image,
	// so checking only 'imageSELinuxMode' is sufficient to determine whether
	// selinux is enabled or not for this image (regardless of the source of
	// that configuration).
	disableSELinux := false
	if artifactsStore.info.seLinuxMode != imagecustomizerapi.SELinuxModeDisabled {
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

	// Update grub.cfg
	err = updateGrubCfg(artifactsStore.files.isoGrubCfgPath, artifactsStore.files.pxeGrubCfgPath, disableSELinux,
		updatedSavedConfigs, filepath.Base(outputImagePath))
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the ISO bootimage (/boot/grub2/efiboot.img)
	artifactsStore.files.isoBootImagePath = filepath.Join(artifactsStore.files.artifactsDir, isoBootImagePath)
	err = isogenerator.BuildIsoBootImage(isoBuildDir, artifactsStore.files.bootEfiPath,
		artifactsStore.files.grubEfiPath, artifactsStore.files.isoBootImagePath)
	if err != nil {
		return fmt.Errorf("failed to build iso boot image:\n%w", err)
	}

	// Generate the initrd image
	outputInitrdPath := filepath.Join(artifactsStore.files.artifactsDir, initrdImage)
	err = createInitrdImage(writeableRootfsDir, artifactsStore.info.kernelVersion, outputInitrdPath)
	if err != nil {
		return fmt.Errorf("failed to create initrd image:\n%w", err)
	}
	artifactsStore.files.initrdImagePath = outputInitrdPath

	// Generate the squashfs image
	outputSquashfsPath := filepath.Join(artifactsStore.files.artifactsDir, liveOSImage)
	err = createSquashfsImage(writeableRootfsDir, outputSquashfsPath)
	if err != nil {
		return fmt.Errorf("failed to create squashfs image:\n%w", err)
	}
	artifactsStore.files.squashfsImagePath = outputSquashfsPath

	// Generate the final iso image
	err = createIsoImageAndPXEFolder(isoBuildDir, baseConfigPath, additionalIsoFiles, artifactsStore, outputImagePath, outputPXEArtifactsDir)
	if err != nil {
		return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
	}

	return nil
}

func createImageFromUnchangedOS(isoBuildDir string, baseConfigPath string, isoConfig *imagecustomizerapi.Iso,
	pxeConfig *imagecustomizerapi.Pxe, inputArtifactsStore *IsoArtifactsStore, outputImagePath string, outputPXEArtifactsDir string) error {

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

	updatedSavedConfigs, err := updateSavedConfigs(inputArtifactsStore.files.savedConfigsFilePath, extraCommandLine, pxeIsoImageBaseUrl,
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

	// Update grub.cfg
	err = updateGrubCfg(inputArtifactsStore.files.isoGrubCfgPath, inputArtifactsStore.files.pxeGrubCfgPath, disableSELinux, updatedSavedConfigs, filepath.Base(outputImagePath))
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the final iso image
	err = createIsoImageAndPXEFolder(isoBuildDir, baseConfigPath, additionalIsoFiles, inputArtifactsStore, outputImagePath, outputPXEArtifactsDir)
	if err != nil {
		return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
	}

	return nil
}

func createIsoImageAndPXEFolder(buildDir string, baseConfigPath string, additionalIsoFiles imagecustomizerapi.AdditionalFileList, artifactsStore *IsoArtifactsStore, outputImagePath string,
	outputPXEArtifactsDir string) error {

	err := createIsoImage(buildDir, artifactsStore.files, baseConfigPath, additionalIsoFiles, outputImagePath)
	if err != nil {
		return fmt.Errorf("failed to create the Iso image.\n%w", err)
	}

	if outputPXEArtifactsDir != "" {
		err = verifyDracutPXESupport(artifactsStore.info.dracutPackageInfo)
		if err != nil {
			return fmt.Errorf("failed to verify Dracut's PXE support.\n%w", err)
		}
		err = populatePXEArtifactsDir(outputImagePath, buildDir, outputPXEArtifactsDir)
		if err != nil {
			return fmt.Errorf("failed to populate the PXE artifacts folder.\n%w", err)
		}
	}

	return nil
}

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
