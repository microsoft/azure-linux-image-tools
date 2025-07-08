// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	defaultIsoImageName = "image.iso"
)

type LiveOSConfig struct {
	isPxe              bool
	kernelCommandLine  imagecustomizerapi.KernelCommandLine
	additionalFiles    imagecustomizerapi.AdditionalFileList
	initramfsType      imagecustomizerapi.InitramfsImageType
	keepKdumpBootFiles bool
	bootstrapBaseUrl   string
	bootstrapFileUrl   string
}

func resolveInitramfsType(inputArtifactsStore *IsoArtifactsStore, outputInitramfsType imagecustomizerapi.InitramfsImageType,
	defaultInitramfsType imagecustomizerapi.InitramfsImageType) (
	resolvedInitramfsType imagecustomizerapi.InitramfsImageType, convertingInitramfsType bool) {
	// if user does not specify initramfs type, and there is an input image
	// , then we should follow the input image.
	var inputInitramfsType imagecustomizerapi.InitramfsImageType
	if inputArtifactsStore != nil {
		if inputArtifactsStore.files.squashfsImagePath != "" {
			inputInitramfsType = imagecustomizerapi.InitramfsImageTypeBootstrap
		} else {
			inputInitramfsType = imagecustomizerapi.InitramfsImageTypeFullOS
		}
	}

	resolvedInitramfsType = outputInitramfsType

	if outputInitramfsType == imagecustomizerapi.InitramfsImageTypeUnspecified {
		// User did not specify initramfsType
		if inputArtifactsStore != nil {
			// Just use the input initramfsType
			resolvedInitramfsType = inputInitramfsType
			// We keep the previous type
			convertingInitramfsType = false
		} else {
			// Just use default
			resolvedInitramfsType = defaultInitramfsType
			// If input is nil, it means the input is not an ISO
			convertingInitramfsType = true
		}
	} else {
		// User did specify initramfsType
		if inputArtifactsStore != nil {
			// Check if it is different from the input
			if inputInitramfsType == outputInitramfsType {
				// We keep the previous type
				convertingInitramfsType = false
			} else {
				// We keep the previous type
				convertingInitramfsType = true
			}
		} else {
			// If input is nil, it means the input is not an ISO
			convertingInitramfsType = true
		}
	}

	return resolvedInitramfsType, convertingInitramfsType
}

func buildLiveOSConfig(inputArtifactsStore *IsoArtifactsStore, isoConfig *imagecustomizerapi.Iso,
	pxeConfig *imagecustomizerapi.Pxe, outputFormat imagecustomizerapi.ImageFormatType) (
	config LiveOSConfig, convertingInitramfsType bool, err error) {
	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		config.isPxe = false
		if isoConfig != nil {
			config.kernelCommandLine = isoConfig.KernelCommandLine
			config.additionalFiles = isoConfig.AdditionalFiles
			config.initramfsType = isoConfig.InitramfsType
			if isoConfig.KdumpBootFiles != nil {
				switch *isoConfig.KdumpBootFiles {
				case imagecustomizerapi.KdumpBootFilesTypeNone:
					config.keepKdumpBootFiles = false
				case imagecustomizerapi.KdumpBootFilesTypeKeep:
					config.keepKdumpBootFiles = true
				default:
					return config, false, fmt.Errorf("invalid kdumpBootFiles value (%s) in ISO configuration", *isoConfig.KdumpBootFiles)
				}
			}
		}

		config.initramfsType, convertingInitramfsType = resolveInitramfsType(inputArtifactsStore, config.initramfsType,
			imagecustomizerapi.InitramfsImageTypeBootstrap)

	case imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		config.isPxe = true
		if pxeConfig != nil {
			config.kernelCommandLine = pxeConfig.KernelCommandLine
			config.additionalFiles = pxeConfig.AdditionalFiles
			config.initramfsType = pxeConfig.InitramfsType
			config.bootstrapBaseUrl = pxeConfig.BootstrapBaseUrl
			config.bootstrapFileUrl = pxeConfig.BootstrapFileUrl
			if pxeConfig.KdumpBootFiles != nil {
				switch *pxeConfig.KdumpBootFiles {
				case imagecustomizerapi.KdumpBootFilesTypeNone:
					config.keepKdumpBootFiles = false
				case imagecustomizerapi.KdumpBootFilesTypeKeep:
					config.keepKdumpBootFiles = true
				default:
					return config, false, fmt.Errorf("invalid kdumpBootFiles value (%s) in PXE configuration", *pxeConfig.KdumpBootFiles)
				}
			}
		}

		config.initramfsType, convertingInitramfsType = resolveInitramfsType(inputArtifactsStore, config.initramfsType,
			imagecustomizerapi.InitramfsImageTypeFullOS)

	default:
		return config, false, fmt.Errorf("unsupported liveos output format (%s)", outputFormat)
	}

	return config, convertingInitramfsType, nil
}

func populateWriteableRootfsDir(sourceDir, writeableRootfsDir string) error {
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

	liveosConfig, _, err := buildLiveOSConfig(inputArtifactsStore, isoConfig, pxeConfig, outputFormat)
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

	liveosConfig, _, err := buildLiveOSConfig(inputArtifactsStore, isoConfig, pxeConfig, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to build live OS configuration from input configuration:\n%w", err)
	}

	err = repackageLiveOSHelper(isoBuildDir, baseConfigPath, liveosConfig, inputArtifactsStore, outputFormat, outputPath)
	if err != nil {
		return fmt.Errorf("failed to create live OS artifacts:\n%w", err)
	}

	return nil
}

func isIsoBootImageNeeded(outputFormat imagecustomizerapi.ImageFormatType, initramfsType imagecustomizerapi.InitramfsImageType) bool {
	if outputFormat == imagecustomizerapi.ImageFormatTypeIso {
		return true
	}
	if (outputFormat == imagecustomizerapi.ImageFormatTypePxeDir || outputFormat == imagecustomizerapi.ImageFormatTypePxeTar) &&
		initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		// Currently, if pxe with bootstrapped image is specified, then the
		// bootstrapped image is an iso. If we support bootstrapped images of
		// other types (like squash file system image), we need to do further
		// checks here before returning true.
		return true
	}
	return false
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
	rawImageConnection, _, _, err := connectToExistingImage(rawImageFile, isoBuildDir, "readonly-rootfs-mount", false /*includeDefaultMounts*/, false)
	if err != nil {
		return err
	}
	defer rawImageConnection.Close()

	// Find out if selinux is enabled
	bootCustomizer, err := NewBootCustomizer(rawImageConnection.Chroot())
	if err != nil {
		return fmt.Errorf("failed to attach to raw image to inspect selinux status:\n%w", err)
	}

	selinuxMode, err := bootCustomizer.GetSELinuxMode(rawImageConnection.Chroot())
	if err != nil {
		return fmt.Errorf("failed to get selinux mode:\n%w", err)
	}
	if (selinuxMode != imagecustomizerapi.SELinuxModeDisabled) && (liveosConfig.initramfsType == imagecustomizerapi.InitramfsImageTypeFullOS) {
		return fmt.Errorf("selinux is not supported for full OS initramfs image")
	}

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
	updatedSavedConfigs, err := updateSavedConfigs(artifactsStore.files.savedConfigsFilePath, liveosConfig.kernelCommandLine,
		liveosConfig.bootstrapBaseUrl, liveosConfig.bootstrapFileUrl, artifactsStore.info.dracutPackageInfo, requestedSelinuxMode,
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
	err = updateGrubCfg(artifactsStore.files.isoGrubCfgPath, outputFormat, liveosConfig.initramfsType, disableSELinux,
		updatedSavedConfigs, getKernelVersions(artifactsStore.files), artifactsStore.files.isoGrubCfgPath,
		artifactsStore.files.pxeGrubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the ISO boot image (/boot/grub2/efiboot.img)
	if isIsoBootImageNeeded(outputFormat, liveosConfig.initramfsType) {
		artifactsStore.files.isoBootImagePath = filepath.Join(artifactsStore.files.artifactsDir, isoBootImagePath)
		err = isogenerator.BuildIsoBootImage(isoBuildDir, artifactsStore.files.bootEfiPath,
			artifactsStore.files.grubEfiPath, artifactsStore.files.isoBootImagePath)
		if err != nil {
			return fmt.Errorf("failed to build iso boot image:\n%w", err)
		}
	}

	switch liveosConfig.initramfsType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		outputInitrdPath := filepath.Join(artifactsStore.files.artifactsDir, initrdImage)
		// Generate the initrd image
		err = createFullOSInitrdImage(writeableRootfsDir,
			liveosConfig.keepKdumpBootFiles, artifactsStore.files.kdumpBootFiles, outputInitrdPath)
		if err != nil {
			return fmt.Errorf("failed to create initrd image:\n%w", err)
		}
		artifactsStore.files.initrdImagePath = outputInitrdPath
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		// Generate the initrd image(s)
		for kernelVersion, kernelBootFiles := range artifactsStore.files.kernelBootFiles {
			err = createBootstrapInitrdImage(writeableRootfsDir, kernelVersion, kernelBootFiles.initrdImagePath)
			if err != nil {
				return fmt.Errorf("failed to create initrd image:\n%w", err)
			}
		}
		artifactsStore.files.initrdImagePath = ""

		// Generate the squashfs image
		outputSquashfsPath := filepath.Join(artifactsStore.files.artifactsDir, liveOSImage)
		err = createSquashfsImage(writeableRootfsDir,
			liveosConfig.keepKdumpBootFiles, artifactsStore.files.kdumpBootFiles, outputSquashfsPath)
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
		err := createIsoImage(isoBuildDir, baseConfigPath, liveosConfig.initramfsType, artifactsStore.files,
			liveosConfig.keepKdumpBootFiles, liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to create the Iso image\n%w", err)
		}
	case imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		err = createPXEArtifacts(isoBuildDir, outputFormat, baseConfigPath, liveosConfig.initramfsType, artifactsStore,
			liveosConfig.keepKdumpBootFiles, liveosConfig.additionalFiles,
			liveosConfig.bootstrapBaseUrl, liveosConfig.bootstrapFileUrl, outputPath)
		if err != nil {
			return fmt.Errorf("failed to generate PXE artifacts\n%w", err)
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
		liveosConfig.bootstrapBaseUrl, liveosConfig.bootstrapFileUrl, nil /*dracut pkg info*/, requestedSelinuxMode,
		nil /*selinux policy pkg info*/)
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
	err = updateGrubCfg(inputArtifactsStore.files.isoGrubCfgPath, outputFormat, liveosConfig.initramfsType, disableSELinux,
		updatedSavedConfigs, getKernelVersions(inputArtifactsStore.files), inputArtifactsStore.files.isoGrubCfgPath,
		inputArtifactsStore.files.pxeGrubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to update grub.cfg:\n%w", err)
	}

	// Generate the final iso image
	switch outputFormat {
	case imagecustomizerapi.ImageFormatTypeIso:
		err := createIsoImage(isoBuildDir, baseConfigPath, liveosConfig.initramfsType, inputArtifactsStore.files,
			liveosConfig.keepKdumpBootFiles, liveosConfig.additionalFiles, outputPath)
		if err != nil {
			return fmt.Errorf("failed to create the Iso image\n%w", err)
		}
	case imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
		err = createPXEArtifacts(isoBuildDir, outputFormat, baseConfigPath, liveosConfig.initramfsType, inputArtifactsStore,
			liveosConfig.keepKdumpBootFiles, liveosConfig.additionalFiles,
			liveosConfig.bootstrapBaseUrl, liveosConfig.bootstrapFileUrl, outputPath)
		if err != nil {
			return fmt.Errorf("failed to generate PXE artifacts folder\n%w", err)
		}
	}

	return nil
}
