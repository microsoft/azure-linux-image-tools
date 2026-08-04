// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/isogenerator"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

const (
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

// blscfgCommandRegex matches the bare `blscfg` command invocation (the line that, at boot, enumerates the BLS entries
// under /boot/loader/entries) on its own line.
var blscfgCommandRegex = regexp.MustCompile(`(?m)^[ \t]*blscfg[ \t]*$`)

// updateLiveOSGrubCfgBLSForLiveOS applies the common LiveOS-compatibility edits for distros that use Boot Loader
// Specification entries.
func updateLiveOSGrubCfgBLSForLiveOS(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, savedConfigs *SavedConfigs,
) (string, error) {
	grubCfgContent, err := replaceSearchCommandAll(grubCfgContent, isogenerator.DefaultVolumeId)
	if err != nil {
		return "", fmt.Errorf("failed to update the search command in the live OS grub.cfg:\n%w", err)
	}

	err = updateLiveOSBLSEntries(bootDir, initramfsType, savedConfigs)
	if err != nil {
		return "", fmt.Errorf("failed to update the live OS BLS entries:\n%w", err)
	}
	return grubCfgContent, nil
}

// updateLiveOSGrubCfgBLSForIso applies the iso-specific root rewrite (root=live:LABEL for a bootstrap initramfs) to
// the BLS .conf files under bootDir for BLS distros.
func updateLiveOSGrubCfgBLSForIso(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType,
) (string, error) {
	if initramfsType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		rootValue := fmt.Sprintf(rootValueLiveOSTemplate, isogenerator.DefaultVolumeId)
		err := setLiveOSBLSEntriesRoot(bootDir, rootValue, nil)
		if err != nil {
			return "", fmt.Errorf("failed to update the root kernel argument in the iso BLS entries:\n%w", err)
		}
	}
	return grubCfgContent, nil
}

// updateLiveOSGrubCfgBLSForPxe applies the pxe-specific grub.cfg edits (removing 'search') for BLS distros.
func updateLiveOSGrubCfgBLSForPxe(grubCfgContent string) (string, error) {
	grubCfgContent, err := removeCommandAll(grubCfgContent, "search")
	if err != nil {
		return "", fmt.Errorf("failed to remove the 'search' commands from PXE grub.cfg:\n%w", err)
	}
	return grubCfgContent, nil
}

// finalizeLiveOSPxeBLSEntries writes the pxe-specific root=live:<url> arg (plus the dracut pxe args) to the BLS
// entries under bootDir.
func finalizeLiveOSPxeBLSEntries(bootDir string, initramfsImageType imagecustomizerapi.InitramfsImageType,
	bootstrapBaseUrl string, bootstrapFileUrl string,
) error {
	if initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap {
		fileUrl, err := getPxeBootstrapFileUrl(bootstrapBaseUrl, bootstrapFileUrl)
		if err != nil {
			return err
		}

		rootValue := fmt.Sprintf(rootValuePxeTemplate, fileUrl)
		err = setLiveOSBLSEntriesRoot(bootDir, rootValue, strings.Fields(pxeBootstrapKernelsArgs))
		if err != nil {
			return err
		}
	}

	// The BLS entries now carry their final kernel command line, so expand them into explicit grub menuentries.
	return expandBLSEntriesToPxeGrubMenu(bootDir)
}

// expandBLSEntriesToPxeGrubMenu rewrites the staged PXE grub.cfg so it carries explicit menuentries instead of relying
// on the `blscfg` command.
func expandBLSEntriesToPxeGrubMenu(bootDir string) error {
	menuEntries, err := renderGrubMenuEntriesFromBLS(bootDir)
	if err != nil {
		return err
	}
	if menuEntries == "" {
		return fmt.Errorf("found no BLS entries under (%s) to build the PXE grub menu",
			filepath.Join(bootDir, "loader", "entries"))
	}

	grubCfgPath := filepath.Join(bootDir, "grub2", isoGrubCfg)
	content, err := file.Read(grubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to read PXE grub.cfg (%s):\n%w", grubCfgPath, err)
	}

	if !blscfgCommandRegex.MatchString(content) {
		return fmt.Errorf("expected a 'blscfg' command in PXE grub.cfg (%s) but found none", grubCfgPath)
	}

	// Boot the first entry deterministically: over PXE the grubenv is not writable, so the header's
	// 'set default="${saved_entry}"' resolves to an empty value.
	replacement := "set default=0\n" + strings.TrimRight(menuEntries, "\n")

	// Use the literal replacement form so '$' in a kernel command line is not treated as a regexp group reference.
	newContent := blscfgCommandRegex.ReplaceAllLiteralString(content, replacement)

	err = file.Write(newContent, grubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to write PXE grub.cfg (%s):\n%w", grubCfgPath, err)
	}

	return nil
}

func updateGrubCfgForLiveOS(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType,
	savedConfigs *SavedConfigs, kernelVersions []string,
) (string, error) {
	inputContentString, err := replaceSearchCommandAll(inputContentString, isogenerator.DefaultVolumeId)
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

	savedArgs := GrubArgsToString(savedConfigs.LiveOS.KernelCommandLine.ExtraCommandLine)
	additionalKernelCommandline := liveosKernelArgs + " " + savedArgs

	inputContentString, err = appendKernelCommandLineArgsAll(inputContentString, additionalKernelCommandline)
	if err != nil {
		return "", fmt.Errorf("failed to update the kernel arguments with the LiveOS configuration and user configuration in the live OS grub.cfg:\n%w", err)
	}

	return inputContentString, nil
}

func updateGrubCfgForIso(inputContentString string, initramfsImageType imagecustomizerapi.InitramfsImageType) (outputContentString string, err error) {
	switch initramfsImageType {
	case imagecustomizerapi.InitramfsImageTypeFullOS:
		// No changes
		outputContentString = inputContentString
	case imagecustomizerapi.InitramfsImageTypeBootstrap:
		// Update 'root'
		rootValue := fmt.Sprintf(rootValueLiveOSTemplate, isogenerator.DefaultVolumeId)
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
	bootstrapFileUrl string,
) (string, error) {
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
func updateGrubCfg(inputGrubCfgPath string, outputFormat imagecustomizerapi.ImageFormatType, initramfsImageType imagecustomizerapi.InitramfsImageType,
	savedConfigs *SavedConfigs, kernelVersions []string, outputIsoGrubCfgPath, outputPxeGrubCfgPath string,
	distroHandler DistroHandler,
) error {
	logger.Log.Infof("Updating grub.cfg")

	inputContentString, err := file.Read(inputGrubCfgPath)
	if err != nil {
		return err
	}

	bootDir := filepath.Dir(filepath.Dir(inputGrubCfgPath))

	// Update grub.cfg content to be 'live-os compatible'.
	liveosContentString, err := distroHandler.UpdateLiveOSGrubCfgForLiveOS(inputContentString, bootDir,
		initramfsImageType, savedConfigs, kernelVersions)
	if err != nil {
		return err
	}

	// Update grub.cfg content to be used for iso booting.
	if (outputFormat == imagecustomizerapi.ImageFormatTypeIso) ||
		((outputFormat == imagecustomizerapi.ImageFormatTypePxeDir || outputFormat == imagecustomizerapi.ImageFormatTypePxeTar) &&
			initramfsImageType == imagecustomizerapi.InitramfsImageTypeBootstrap) {
		isoContentString, err := distroHandler.UpdateLiveOSGrubCfgForIso(liveosContentString, bootDir,
			initramfsImageType)
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
		pxeContentString, err := distroHandler.UpdateLiveOSGrubCfgForPxe(liveosContentString, initramfsImageType,
			savedConfigs.Pxe.bootstrapBaseUrl, savedConfigs.Pxe.bootstrapFileUrl)
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
