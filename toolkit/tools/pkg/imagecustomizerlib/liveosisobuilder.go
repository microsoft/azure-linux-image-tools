// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	defaultIsoImageName = "image.iso"
)

type LiveOSConfig struct {
	isPxe             bool
	kernelCommandLine imagecustomizerapi.KernelCommandLine
	additionalFiles   imagecustomizerapi.AdditionalFileList
	initramfsType     imagecustomizerapi.InitramfsImageType
	bootstrapBaseUrl  string
	bootstrapFileUrl  string
}

func buildLiveOSConfig(outputFormat imagecustomizerapi.ImageFormatType, isoConfig *imagecustomizerapi.Iso, pxeConfig *imagecustomizerapi.Pxe) (
	config LiveOSConfig, err error) {

	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		config.isPxe = false
		if isoConfig != nil {
			config.kernelCommandLine = isoConfig.KernelCommandLine
			config.additionalFiles = isoConfig.AdditionalFiles
			config.initramfsType = isoConfig.InitramfsType
		}
		// Set default initramfs type
		if config.initramfsType == imagecustomizerapi.InitramfsImageTypeUnspecified {
			config.initramfsType = imagecustomizerapi.InitramfsImageTypeBootstrap
		}
	case imagecustomizerapi.ImageFormatTypePxe:
		config.isPxe = true
		if pxeConfig != nil {
			config.kernelCommandLine = pxeConfig.KernelCommandLine
			config.additionalFiles = pxeConfig.AdditionalFiles
			config.initramfsType = pxeConfig.InitramfsType
			config.bootstrapBaseUrl = pxeConfig.BootstrapBaseUrl
			config.bootstrapFileUrl = pxeConfig.BootstrapFileUrl
		}
		// Set default initramfs type
		if config.initramfsType == imagecustomizerapi.InitramfsImageTypeUnspecified {
			config.initramfsType = imagecustomizerapi.InitramfsImageTypeFullOS
		}
	default:
		return config, fmt.Errorf("unsupported liveos output format (%s)", outputFormat)
	}

	return config, nil
}

func populateWriteableRootfsDir(sourceDir, writeableRootfsDir string) error {

	logger.Log.Infof("Creating writeable rootfs (%s) from (%s)", writeableRootfsDir, sourceDir)

	err := os.MkdirAll(writeableRootfsDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder (%s):\n%w", writeableRootfsDir, err)
	}

	err = copyPartitionFiles(sourceDir+"/.", writeableRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to copy rootfs contents to a writeable folder (%s):\n%w", writeableRootfsDir, err)
	}

	return nil
}

func createLiveOSFromRaw(buildDir, baseConfigPath string, inputArtifactsStore *IsoArtifactsStore, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
	isoConfig *imagecustomizerapi.Iso, pxeConfig *imagecustomizerapi.Pxe, rawImageFile string, outputFormat imagecustomizerapi.ImageFormatType,
	outputPath string,
) (err error) {
	logger.Log.Infof("Creating Live OS artifacts using customized full OS image")

	liveosConfig, err := buildLiveOSConfig(outputFormat, isoConfig, pxeConfig)
	if err != nil {
		return fmt.Errorf("failed to build live OS configuration from input configuration:\n%w", err)
	}

	err = createLiveOSFromRawHelper(buildDir, baseConfigPath, inputArtifactsStore, requestedSelinuxMode, liveosConfig, rawImageFile, outputFormat, outputPath)
	if err != nil {
		return fmt.Errorf("failed to create live OS artifacts:\n%w", err)
	}

	return nil
}

func repackageLiveOS(isoBuildDir string, baseConfigPath string, isoConfig *imagecustomizerapi.Iso, pxeConfig *imagecustomizerapi.Pxe,
	inputArtifactsStore *IsoArtifactsStore, outputFormat imagecustomizerapi.ImageFormatType, outputPath string,
) error {
	logger.Log.Infof("Creating Live OS artifacts using input ISO image")

	liveosConfig, err := buildLiveOSConfig(outputFormat, isoConfig, pxeConfig)
	if err != nil {
		return fmt.Errorf("failed to build live OS configuration from input configuration:\n%w", err)
	}

	err = repackageLiveOSHelper(isoBuildDir, baseConfigPath, liveosConfig, inputArtifactsStore, outputFormat, outputPath)
	if err != nil {
		return fmt.Errorf("failed to create live OS artifacts:\n%w", err)
	}

	return nil
}

func createLiveOSFromRawHelper(buildDir, baseConfigPath string, inputArtifactsStore *IsoArtifactsStore, requestedSelinuxMode imagecustomizerapi.SELinuxMode,
	liveosConfig LiveOSConfig, rawImageFile string, outputFormat imagecustomizerapi.ImageFormatType,
	outputPath string,
) (err error) {
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
	rawImageConnection, _, _, err := connectToExistingImage(rawImageFile, isoBuildDir, "readonly-rootfs-mount", false /*includeDefaultMounts*/)
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
	updatedSavedConfigs, err := updateSavedConfigs(artifactsStore.files.savedConfigsFilePath, liveosConfig.kernelCommandLine, liveosConfig.bootstrapBaseUrl,
		liveosConfig.bootstrapFileUrl, artifactsStore.info.kernelVersion, artifactsStore.info.dracutPackageInfo, requestedSelinuxMode,
		artifactsStore.info.selinuxPolicyPackageInfo)
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
	err = updateGrubCfg(outputFormat, liveosConfig.initramfsType, artifactsStore.files.isoGrubCfgPath, disableSELinux,
		updatedSavedConfigs, filepath.Base(outputPath))
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the ISO boot image (/boot/grub2/efiboot.img)
	artifactsStore.files.isoBootImagePath = filepath.Join(artifactsStore.files.artifactsDir, isoBootImagePath)
	err = isogenerator.BuildIsoBootImage(isoBuildDir, artifactsStore.files.bootEfiPath,
		artifactsStore.files.grubEfiPath, artifactsStore.files.isoBootImagePath)
	if err != nil {
		return fmt.Errorf("failed to build iso boot image:\n%w", err)
	}

	outputInitrdPath := filepath.Join(artifactsStore.files.artifactsDir, initrdImage)

	switch liveosConfig.initramfsType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		// Generate the initrd image
		err = createFullOSInitrdImage(writeableRootfsDir, outputInitrdPath)
		if err != nil {
			return fmt.Errorf("failed to create initrd image:\n%w", err)
		}
		artifactsStore.files.initrdImagePath = outputInitrdPath
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		// Generate the initrd image
		err = createBootstrapInitrdImage(writeableRootfsDir, artifactsStore.info.kernelVersion, outputInitrdPath)
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
	default:
		return fmt.Errorf("unsupported initramfs type (%s)", liveosConfig.initramfsType)
	}

	// Generate the final output artifacts
	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		err := createIsoImage(isoBuildDir, baseConfigPath, artifactsStore.files, liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to create the Iso image.\n%w", err)
		}
	case imagecustomizerapi.ImageFormatTypePxe:
		err = createPXEArtifacts(isoBuildDir, baseConfigPath, liveosConfig.initramfsType, artifactsStore,
			liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
		}
	}

	return nil
}

func repackageLiveOSHelper(isoBuildDir string, baseConfigPath string, liveosConfig LiveOSConfig, inputArtifactsStore *IsoArtifactsStore,
	outputFormat imagecustomizerapi.ImageFormatType, outputPath string,
) error {
	// Note that in this ISO build flow, there is no os configuration, and hence
	// no selinux configuration. So, we will set it to default (i.e. unspecified)
	// and let any saved data override if present.
	requestedSelinuxMode := imagecustomizerapi.SELinuxModeDefault

	updatedSavedConfigs, err := updateSavedConfigs(inputArtifactsStore.files.savedConfigsFilePath, liveosConfig.kernelCommandLine,
		liveosConfig.bootstrapBaseUrl, liveosConfig.bootstrapFileUrl, inputArtifactsStore.info.kernelVersion,
		nil /*dracut pkg info*/, requestedSelinuxMode, nil /*selinux policy pkg info*/)
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
	err = updateGrubCfg(outputFormat, liveosConfig.initramfsType, inputArtifactsStore.files.isoGrubCfgPath,
		disableSELinux, updatedSavedConfigs, filepath.Base(outputPath))
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the final iso image
	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		err := createIsoImage(isoBuildDir, baseConfigPath, inputArtifactsStore.files, liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to create the Iso image.\n%w", err)
		}
	case imagecustomizerapi.ImageFormatTypePxe:
		err = createPXEArtifacts(isoBuildDir, baseConfigPath, liveosConfig.initramfsType, inputArtifactsStore,
			liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to generate iso image and/or PXE artifacts folder\n%w", err)
		}
	}

	return nil
}

func createPXEArtifacts(buildDir string, baseConfigPath string, initramfsType imagecustomizerapi.InitramfsImageType,
	artifactsStore *IsoArtifactsStore, additionalIsoFiles imagecustomizerapi.AdditionalFileList, outputPath string) (err error) {
	logger.Log.Infof("Creating PXE output at (%s)", outputPath)

	outputPXEArtifactsDir := ""
	outputPXEImage := ""

	if strings.HasSuffix(outputPath, ".tar.gz") {
		// Output is a .tar.gz
		outputPXEArtifactsDir, err = os.MkdirTemp(buildDir, "tmp-pxe-")
		if err != nil {
			return fmt.Errorf("failed to create temporary mount folder for squashfs:\n%w", err)
		}
		defer os.RemoveAll(outputPXEArtifactsDir)
		outputPXEImage = outputPath
	} else {
		// Output is a folder
		outputPXEArtifactsDir = outputPath
		err := os.MkdirAll(outputPXEArtifactsDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create folder (%s):\n%w", outputPXEArtifactsDir, err)
		}
		outputPXEImage = ""
	}

	outputISODir, err := os.MkdirTemp(buildDir, "tmp-pxe-")
	if err != nil {
		return fmt.Errorf("failed to create temp folder:\n%w", err)
	}
	defer os.RemoveAll(outputISODir)

	isoImagePath := filepath.Join(outputISODir, defaultIsoImageName)
	err = createIsoImage(buildDir, baseConfigPath, artifactsStore.files, additionalIsoFiles, isoImagePath)
	if err != nil {
		return fmt.Errorf("failed to create the Iso image.\n%w", err)
	}

	err = verifyDracutPXESupport(artifactsStore.info.dracutPackageInfo)
	if err != nil {
		return fmt.Errorf("failed to verify Dracut's PXE support.\n%w", err)
	}

	// Extract all files from the iso image file.
	err = extractIsoImageContents(buildDir, isoImagePath, outputPXEArtifactsDir)
	if err != nil {
		return err
	}

	// Move bootloader files from under '<pxe-folder>/efi/boot' to '<pxe-folder>/'
	_, bootFilesConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}
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

	if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		// The iso image file itself must be placed in the PXE folder because
		// dracut livenet module will download it.
		artifactsIsoImagePath := filepath.Join(outputPXEArtifactsDir, filepath.Base(isoImagePath))
		err = file.Move(isoImagePath, artifactsIsoImagePath)
		if err != nil {
			return fmt.Errorf("failed to copy (%s) while populating the PXE artifacts directory:\n%w", isoImagePath, err)
		}
	} else {
		err = os.Remove(isoImagePath)
		if err != nil {
			return fmt.Errorf("failed to remove (%s) while cleaning up intermediate files:\n%w", isoImagePath, err)
		}
	}

	if outputPXEImage != "" {
		err = createTarGzArchive(outputPXEArtifactsDir, outputPXEImage)
		if err != nil {
			return fmt.Errorf("failed to create archive (%s) from (%s):\n%w", outputPXEImage, outputPXEArtifactsDir, err)
		}

		err = os.RemoveAll(outputPXEArtifactsDir)
		if err != nil {
			return fmt.Errorf("failed to remove (%s) while cleaning up intermediate files:\n%w", outputPXEArtifactsDir, err)
		}
	}

	return nil
}
