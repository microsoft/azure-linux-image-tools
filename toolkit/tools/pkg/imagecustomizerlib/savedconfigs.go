// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
)

// 'SavedConfigs' is a subset of the Image Customizer input configurations that
// needs to be saved on the output media so that it can be used in subsequent
// runs of the Image Customizer against that same output media.
//
// This preservation of input configuration is necessary for subsequent runs if
// the configuration does not result in updating root file system.
//
// For example, if the user specifies a kernel argument that is specific to the
// ISO image, it will not be present in any of the grub config files on the
// root file system - only in the final ISO image grub.cfg. When that ISO image
// is further customized, the root file system grub.cfg might get re-generated
// and we need to remember to add the ISO specific arguments from the previous
// runs. SavedConfigs is the place where we can store such arguments so we can
// re-apply them.

type IsoSavedConfigs struct {
	KernelCommandLine imagecustomizerapi.KernelCommandLine `yaml:"kernelCommandLine"`
}

func (i *IsoSavedConfigs) IsValid() error {
	err := i.KernelCommandLine.IsValid()
	if err != nil {
		return fmt.Errorf("invalid kernelCommandLine: %w", err)
	}

	return nil
}

type PxeSavedConfigs struct {
	IsoImageBaseUrl string `yaml:"isoImageBaseUrl"`
	IsoImageFileUrl string `yaml:"isoImageFileUrl"`
}

func (p *PxeSavedConfigs) IsValid() error {
	if p.IsoImageBaseUrl != "" && p.IsoImageFileUrl != "" {
		return fmt.Errorf("cannot specify both 'isoImageBaseUrl' and 'isoImageFileUrl' at the same time.")
	}
	err := imagecustomizerapi.IsValidPxeUrl(p.IsoImageBaseUrl)
	if err != nil {
		return err
	}
	err = imagecustomizerapi.IsValidPxeUrl(p.IsoImageFileUrl)
	if err != nil {
		return err
	}
	return nil
}

type OSSavedConfigs struct {
	DracutPackageInfo        *PackageVersionInformation     `yaml:"dracutPackage"`
	RequestedSELinuxMode     imagecustomizerapi.SELinuxMode `yaml:"selinuxRequestedMode"`
	SELinuxPolicyPackageInfo *PackageVersionInformation     `yaml:"selinuxPolicyPackage"`
}

func (i *OSSavedConfigs) IsValid() error {
	return nil
}

type SavedConfigs struct {
	Iso IsoSavedConfigs `yaml:"iso"`
	Pxe PxeSavedConfigs `yaml:"pxe"`
	OS  OSSavedConfigs  `yaml:"os"`
}

func (c *SavedConfigs) IsValid() (err error) {
	err = c.Iso.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'iso' field:\n%w", err)
	}

	err = c.Pxe.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'pxe' field:\n%w", err)
	}

	err = c.OS.IsValid()
	if err != nil {
		return fmt.Errorf("invalud 'os' field:\n%w", err)
	}

	return nil
}

func (c *SavedConfigs) persistSavedConfigs(savedConfigsFilePath string) (err error) {
	err = os.MkdirAll(filepath.Dir(savedConfigsFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory for (%s):\n%w", savedConfigsFilePath, err)
	}

	err = imagecustomizerapi.MarshalYamlFile(savedConfigsFilePath, c)
	if err != nil {
		return fmt.Errorf("failed to persist saved configs file to (%s):\n%w", savedConfigsFilePath, err)
	}

	return nil
}

func loadSavedConfigs(savedConfigsFilePath string) (savedConfigs *SavedConfigs, err error) {
	exists, err := file.PathExists(savedConfigsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if (%s) exists:\n%w", savedConfigsFilePath, err)
	}

	if !exists {
		return nil, nil
	}

	savedConfigs = &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to load saved configs file (%s):\n%w", savedConfigsFilePath, err)
	}

	return savedConfigs, nil
}

func updateSavedConfigs(savedConfigsFilePath string, newKernelArgs []string,
	newPxeIsoImageBaseUrl string, newPxeIsoImageFileUrl string, newDracutPackageInfo *PackageVersionInformation,
	newRequestedSelinuxMode imagecustomizerapi.SELinuxMode, newSELinuxPackageInfo *PackageVersionInformation,
) (outputConfigs *SavedConfigs, err error) {
	outputConfigs = &SavedConfigs{}
	outputConfigs.Iso.KernelCommandLine.ExtraCommandLine = newKernelArgs
	outputConfigs.Pxe.IsoImageBaseUrl = newPxeIsoImageBaseUrl
	outputConfigs.Pxe.IsoImageFileUrl = newPxeIsoImageFileUrl
	outputConfigs.OS.DracutPackageInfo = newDracutPackageInfo
	outputConfigs.OS.RequestedSELinuxMode = newRequestedSelinuxMode
	outputConfigs.OS.SELinuxPolicyPackageInfo = newSELinuxPackageInfo

	inputConfigs, err := loadSavedConfigs(savedConfigsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load saved configurations (%s):\n%w", savedConfigsFilePath, err)
	}

	if inputConfigs != nil {
		// do we have kernel arguments from a previous run?
		if len(inputConfigs.Iso.KernelCommandLine.ExtraCommandLine) > 0 {
			// If yes, add them before the new kernel arguments.
			savedArgs := inputConfigs.Iso.KernelCommandLine.ExtraCommandLine
			newArgs := newKernelArgs

			// Combine saved arguments with new ones
			combinedArgs := append(savedArgs, newArgs...)
			outputConfigs.Iso.KernelCommandLine.ExtraCommandLine = combinedArgs
		}

		// if the PXE iso image url is not set, set it to the value from the previous run.
		if newPxeIsoImageBaseUrl == "" && inputConfigs.Pxe.IsoImageBaseUrl != "" {
			outputConfigs.Pxe.IsoImageBaseUrl = inputConfigs.Pxe.IsoImageBaseUrl
		}

		if newPxeIsoImageFileUrl == "" && inputConfigs.Pxe.IsoImageFileUrl != "" {
			outputConfigs.Pxe.IsoImageFileUrl = inputConfigs.Pxe.IsoImageFileUrl
		}

		// if IsoImageBaseUrl is being set in this run (i.e. newPxeIsoImageBaseUrl != ""),
		// then make sure IsoImageFileUrl is unset (since both fields must be mutually
		// exclusive) - and vice versa.
		if newPxeIsoImageBaseUrl != "" {
			outputConfigs.Pxe.IsoImageFileUrl = ""
		}

		if newPxeIsoImageFileUrl != "" {
			outputConfigs.Pxe.IsoImageBaseUrl = ""
		}

		// newOSDracutVersion can be nil if the input is an ISO and the
		// configuration does not specify OS changes.
		// In such cases, the rootfs is intentionally not expanded (to save
		// time), and Dracut package information will not be retrieved from
		// there. Instead, we use the saved configuration which already has the
		// the dracut version.
		if newDracutPackageInfo == nil {
			outputConfigs.OS.DracutPackageInfo = inputConfigs.OS.DracutPackageInfo
		}
		if newRequestedSelinuxMode != imagecustomizerapi.SELinuxModeDefault {
			outputConfigs.OS.RequestedSELinuxMode = inputConfigs.OS.RequestedSELinuxMode
		}
		if newSELinuxPackageInfo == nil {
			outputConfigs.OS.SELinuxPolicyPackageInfo = inputConfigs.OS.SELinuxPolicyPackageInfo
		}
	}

	err = outputConfigs.persistSavedConfigs(savedConfigsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to save iso configs:\n%w", err)
	}

	return outputConfigs, nil
}
