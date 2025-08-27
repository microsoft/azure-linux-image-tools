// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

const (
	searchCommandTemplate   = "search --label %s --set root"
	rootValueLiveOSTemplate = "live:LABEL=%s"
	rootValuePxeTemplate    = "live:%s"

	// kernel arguments template
	kernelArgsLiveOSTemplate = " rd.shell rd.live.image rd.live.dir=%s rd.live.squashimg=%s rd.live.overlay=1 rd.live.overlay.overlayfs rd.live.overlay.nouserconfirmprompt "

	// PXE kernel arguments
	pxeBootstrapKernelsArgs = "ip=dhcp rd.live.azldownloader=enable"

	liveOSDir       = "liveos"
	liveOSImage     = "rootfs.img"
	liveOSImagePath = "/" + liveOSDir + "/" + liveOSImage

	vmLinuxPathAzl2Template = "/boot/vmlinuz-%s"
	initrdPathAzl2Template  = "/boot/initrd.img-%s"
)

func updateGrubCfgForLiveOS(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType,
	volumeId string, disableSELinux bool, savedConfigs *SavedConfigs, kernelVersions []string) (string, error) {
	searchCommand := fmt.Sprintf(searchCommandTemplate, volumeId)
	inputContentString, err := replaceSearchCommandAll(inputContentString, searchCommand)
	if err != nil {
		return "", fmt.Errorf("failed to update the search command in the live OS grub.cfg:\n%w", err)
	}

	grubMkconfigEnabled := isGrubMkconfigConfig(inputContentString)
	if !grubMkconfigEnabled {
		kernelCount := len(kernelVersions)
		if kernelCount != 1 {
			return "", fmt.Errorf("unsupported number of kernels (%d) installed", kernelCount)
		}

		vmLinuzPath := fmt.Sprintf(vmLinuxPathAzl2Template, kernelVersions[0])

		var oldLinuxPath string
		inputContentString, oldLinuxPath, err = setLinuxPath(inputContentString, vmLinuzPath)
		if err != nil {
			return "", fmt.Errorf("failed to update the kernel file path in the live OS grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldLinuxPath, vmLinuzPath)
		if err != nil {
			return "", fmt.Errorf("failed to update all the kernel file path occurances in the live OS grub.cfg:\n%w", err)
		}

		initrdPath := isoInitrdPath
		if initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			initrdPath = fmt.Sprintf(initrdPathAzl2Template, kernelVersions[0])
		}

		var oldInitrdPath string
		inputContentString, oldInitrdPath, err = setInitrdPath(inputContentString, initrdPath)
		if err != nil {
			return "", fmt.Errorf("failed to update the initrd file path in the live OS grub.cfg:\n%w", err)
		}

		inputContentString, err = replaceToken(inputContentString, oldInitrdPath, initrdPath)
		if err != nil {
			return "", fmt.Errorf("failed to update all the initrd file path occurances in the live OS grub.cfg:\n%w", err)
		}
	} else {
		// update the initrd path from /vmlinux-<version> to /boot/vmlinux-<version>
		inputContentString, _, err = prependLinuxOrInitrdPathAll(inputContentString, linuxCommand, isoKernelDir, true /*allowMultiple*/)
		if err != nil {
			return "", fmt.Errorf("failed to update the kernel file path in the live OS grub.cfg:\n%w", err)
		}
	}

	liveosKernelArgs := ""
	switch initramfsImageType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		// update the initrd path from /initrd-<version>.img to /boot/initrd.img
		inputContentString, _, err = setLinuxOrInitrdPathAll(inputContentString, initrdCommand, isoInitrdPath, true /*allowMultiple*/)
		if err != nil {
			return "", fmt.Errorf("failed to update the initrd file path in the live OS grub.cfg:\n%w", err)
		}

		// Remove 'root' so that no pivoting takes place.
		argsToRemove := []string{"root"}
		newArgs := []string{}
		inputContentString, err = updateKernelCommandLineArgsAll(inputContentString, argsToRemove, newArgs)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument in the live OS grub.cfg:\n%w", err)
		}
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		// update the initrd path from /initrd-<version>.img to /boot/initrd-<version>.img
		inputContentString, _, err = prependLinuxOrInitrdPathAll(inputContentString, initrdCommand, isoKernelDir, true /*allowMultiple*/)
		if err != nil {
			return "", fmt.Errorf("failed to update the initrd file path in the live OS grub.cfg:\n%w", err)
		}

		// Add Dracut live os parameters
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

	savedArgs := GrubArgsToString(savedConfigs.LiveOS.KernelCommandLine.ExtraCommandLine)
	additionalKernelCommandline := liveosKernelArgs + " " + savedArgs

	inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, additionalKernelCommandline)
	if err != nil {
		return "", fmt.Errorf("failed to update the kernel arguments with the LiveOS configuration and user configuration in the live OS grub.cfg:\n%w", err)
	}

	inputContentString = strings.Replace(inputContentString, "timeout=0", "timeout=10", 1)

	return inputContentString, nil
}

func updateGrubCfgForIso(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType,
	volumeId string) (outputContentString string, err error) {

	switch initramfsImageType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		// No changes
		outputContentString = inputContentString
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		// Update 'root'
		rootValue := fmt.Sprintf(rootValueLiveOSTemplate, volumeId)
		argsToRemove := []string{"root"}
		newArgs := []string{"root=" + rootValue}
		outputContentString, err = updateKernelCommandLineArgsAll(inputContentString, argsToRemove, newArgs)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument in the iso grub.cfg:\n%w", err)
		}
	default:
		return "", fmt.Errorf("unsupported initramfs image type (%s)", initramfsImageType)
	}
	return outputContentString, nil
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
		inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, pxeBootstrapKernelsArgs)
		if err != nil {
			return "", fmt.Errorf("failed to append the kernel arguments (%s) in the PXE grub.cfg:\n%w", pxeBootstrapKernelsArgs, err)
		}
	}

	return inputContentString, nil
}

// Because of the cumulative nature of our grub modification API, it is simpler
// that we avoid multiple modification passes where we would end with new
// kernel parameters added multiple times.
// This function generates both the iso and the pxe versions of the grub so
// that the call does not need to call it multiple times.
func updateGrubCfg(inputGrubCfgPath string, outputFormat imagecustomizerapi.ImageFormatType, volumeId string,
	initramfsImageType imagecustomizerapi.InitramfsImageType, disableSELinux bool, savedConfigs *SavedConfigs,
	kernelVersions []string, outputIsoGrubCfgPath, outputPxeGrubCfgPath string) error {
	logger.Log.Infof("Updating grub.cfg")

	inputContentString, err := file.Read(inputGrubCfgPath)
	if err != nil {
		return err
	}

	// Update grub.cfg content to be 'live-os compatible'.
	liveosContentString, err := updateGrubCfgForLiveOS(inputContentString, initramfsImageType, volumeId,
		disableSELinux, savedConfigs, kernelVersions)
	if err != nil {
		return err
	}

	// Update grub.cfg content to be used for iso booting.
	if (outputFormat == imagecustomizerapi.ImageFormatTypeIso) ||
		((outputFormat == imagecustomizerapi.ImageFormatTypePxeDir || outputFormat == imagecustomizerapi.ImageFormatTypePxeTar) &&
			initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap) {
		isoContentString, err := updateGrubCfgForIso(liveosContentString, initramfsImageType, volumeId)
		if err != nil {
			return fmt.Errorf("failed to update %s:\n%w", inputGrubCfgPath, err)
		}

		err = file.Write(isoContentString, outputIsoGrubCfgPath)
		if err != nil {
			return fmt.Errorf("failed to write %s:\n%w", outputIsoGrubCfgPath, err)
		}
	}

	// Update grub.cfg content to be used for pxe booting.
	if (outputFormat == imagecustomizerapi.ImageFormatTypePxeDir) ||
		(outputFormat == imagecustomizerapi.ImageFormatTypePxeTar) {
		if initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap {
			// Check if the dracut version in use meets our minimum requirements for
			// PXE support.
			err = verifyDracutPXESupport(savedConfigs.OS.DracutPackageInfo)
			if err != nil {
				return fmt.Errorf("cannot generate grub.cfg for PXE booting.\n%v", err)
			}
		}
		pxeContentString, err := updateGrubCfgForPxe(liveosContentString, initramfsImageType, savedConfigs.Pxe.bootstrapBaseUrl,
			savedConfigs.Pxe.bootstrapFileUrl)
		if err != nil {
			return fmt.Errorf("failed to create grub configuration for PXE booting.\n%w", err)
		}
		err = file.Write(pxeContentString, outputPxeGrubCfgPath)
		if err != nil {
			return fmt.Errorf("failed to write %s:\n%w", outputPxeGrubCfgPath, err)
		}
	}

	return nil
}
