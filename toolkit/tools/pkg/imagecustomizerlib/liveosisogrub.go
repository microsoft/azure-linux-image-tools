// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	searchCommandTemplate   = "search --label %s --set root"
	rootValueLiveOSTemplate = "live:LABEL=%s"
	rootValuePxeTemplate    = "live:%s"

	// kernel arguments template
	kernelArgsLiveOSTemplate = " rd.shell rd.live.image rd.live.dir=%s rd.live.squashimg=%s rd.live.overlay=1 rd.live.overlay.overlayfs rd.live.overlay.nouserconfirmprompt "

	// PXE kernel arguments
	pxeKernelsArgs = "ip=dhcp rd.live.azldownloader=enable"

	liveOSDir       = "liveos"
	liveOSImage     = "rootfs.img"
	liveOSImagePath = "/" + liveOSDir + "/" + liveOSImage
)

func updateGrubCfgForLiveOS(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType,
	disableSELinux bool, savedConfigs *SavedConfigs) (string, error) {
	searchCommand := fmt.Sprintf(searchCommandTemplate, isogenerator.DefaultVolumeId)
	inputContentString, err := replaceSearchCommandAll(inputContentString, searchCommand)
	if err != nil {
		return "", fmt.Errorf("failed to update the search command in the iso grub.cfg:\n%w", err)
	}

	grubMkconfigEnabled := isGrubMkconfigConfig(inputContentString)
	if !grubMkconfigEnabled {
		var oldLinuxPath string
		inputContentString, oldLinuxPath, err = setLinuxPath(inputContentString, isoKernelPath)
		if err != nil {
			return "", fmt.Errorf("failed to update the kernel file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldLinuxPath, isoKernelPath)
		if err != nil {
			return "", fmt.Errorf("failed to update all the kernel file path occurances in the iso grub.cfg:\n%w", err)
		}

		var oldInitrdPath string
		inputContentString, oldInitrdPath, err = setInitrdPath(inputContentString, isoInitrdPath)
		if err != nil {
			return "", fmt.Errorf("failed to update the initrd file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldInitrdPath, isoInitrdPath)
		if err != nil {
			return "", fmt.Errorf("failed to update all the initrd file path occurances in the iso grub.cfg:\n%w", err)
		}
	} else {
		inputContentString, _, err = setLinuxOrInitrdPathAll(inputContentString, linuxCommand, isoKernelPath, true /*allowMultiple*/)
		if err != nil {
			return "", fmt.Errorf("failed to update the kernel file path in the iso grub.cfg:\n%w", err)
		}

		inputContentString, _, err = setLinuxOrInitrdPathAll(inputContentString, initrdCommand, isoInitrdPath, true /*allowMultiple*/)
		if err != nil {
			return "", fmt.Errorf("failed to update the initrd file path in the iso grub.cfg:\n%w", err)
		}
	}

	liveosKernelArgs := ""
	switch initramfsImageType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		argsToRemove := []string{"root"}
		newArgs := []string{}
		inputContentString, err = updateKernelCommandLineArgsAll(inputContentString, argsToRemove, newArgs)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument in the iso grub.cfg:\n%w", err)
		}
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		rootValue := fmt.Sprintf(rootValueLiveOSTemplate, isogenerator.DefaultVolumeId)
		argsToRemove := []string{"root"}
		newArgs := []string{"root=" + rootValue}
		inputContentString, err = updateKernelCommandLineArgsAll(inputContentString, argsToRemove, newArgs)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument in the iso grub.cfg:\n%w", err)
		}
		liveosKernelArgs = fmt.Sprintf(kernelArgsLiveOSTemplate, liveOSDir, liveOSImage)
	default:
		return "", fmt.Errorf("unsupported initramfs image type (%s)", initramfsImageType)
	}

	if disableSELinux {
		inputContentString, err = updateSELinuxCommandLineHelperAll(inputContentString,
			imagecustomizerapi.SELinuxModeDisabled)
		if err != nil {
			return "", fmt.Errorf("failed to set SELinux mode:\n%w", err)
		}
	}

	savedArgs := GrubArgsToString(savedConfigs.Iso.KernelCommandLine.ExtraCommandLine)
	additionalKernelCommandline := liveosKernelArgs + " " + savedArgs

	inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, additionalKernelCommandline)
	if err != nil {
		return "", fmt.Errorf("failed to update the kernel arguments with the LiveOS configuration and user configuration in the iso grub.cfg:\n%w", err)
	}

	return inputContentString, nil
}

func updateGrubCfgForPxe(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType, bootstrapBaseUrl string,
	bootstrapFileUrl string) (string, error) {
	// remove 'search' commands from PXE grub.cfg because it is not needed.
	inputContentString, err := removeCommandAll(inputContentString, "search")
	if err != nil {
		return "", fmt.Errorf("failed to remove the 'search' commands from PXE grub.cfg:\n%w", err)
	}

	if initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		bootstrapFileUrl, err = getPxeBootstrapFileUrl(bootstrapBaseUrl, bootstrapFileUrl)
		if err != nil {
			return "", err
		}

		rootValue := fmt.Sprintf(rootValuePxeTemplate, bootstrapFileUrl)
		inputContentString, err = replaceKernelCommandLineArgValueAll(inputContentString, "root", rootValue)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument with the PXE iso image url in the PXE grub.cfg:\n%w", err)
		}
		inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, pxeKernelsArgs)
		if err != nil {
			return "", fmt.Errorf("failed to append the kernel arguments (%s) in the PXE grub.cfg:\n%w", pxeKernelsArgs, err)
		}
	}

	return inputContentString, nil
}

func updateGrubCfg(outputFormat imagecustomizerapi.ImageFormatType, initramfsImageType imagecustomizerapi.InitramfsImageType,
	isoGrubCfgFileName string, disableSELinux bool, savedConfigs *SavedConfigs) error {
	logger.Log.Infof("Updating ISO grub.cfg")

	inputContentString, err := file.Read(isoGrubCfgFileName)
	if err != nil {
		return err
	}

	inputContentString, err = updateGrubCfgForLiveOS(inputContentString, initramfsImageType, disableSELinux, savedConfigs)
	if err != nil {
		return err
	}

	if outputFormat == imagecustomizerapi.ImageFormatTypePxe {
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
			inputContentString, err = updateGrubCfgForPxe(inputContentString, initramfsImageType, savedConfigs.Pxe.bootstrapBaseUrl,
				savedConfigs.Pxe.bootstrapFileUrl)
			if err != nil {
				return fmt.Errorf("failed to create grub configuration for PXE booting.\n%w", err)
			}
		}
	}

	err = file.Write(inputContentString, isoGrubCfgFileName)
	if err != nil {
		return fmt.Errorf("failed to write %s:\n%w", isoGrubCfgFileName, err)
	}

	return nil
}
