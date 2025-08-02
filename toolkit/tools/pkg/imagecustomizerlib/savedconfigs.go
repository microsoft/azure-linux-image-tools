// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

var (
	// Config validation errors
	ErrConfigInvalidKdumpBootFiles    = NewImageCustomizerError("Config:InvalidKdumpBootFiles", "invalid kdumpBootFiles")
	ErrConfigInvalidKernelCommandLine = NewImageCustomizerError("Config:InvalidKernelCommandLine", "invalid kernelCommandLine")
	ErrConfigBootstrapUrl             = NewImageCustomizerError("Config:BootstrapUrl", "cannot specify both 'bootstrapBaseUrl' and 'bootstrapFileUrl'")
	ErrConfigInvalidIsoField          = NewImageCustomizerError("Config:InvalidIsoField", "invalid 'iso' field")
	ErrConfigInvalidPxeField          = NewImageCustomizerError("Config:InvalidPxeField", "invalid 'pxe' field")
	ErrConfigInvalidOsField           = NewImageCustomizerError("Config:InvalidOsField", "invalid 'os' field")
	ErrConfigDirectoryCreate          = NewImageCustomizerError("Config:DirectoryCreate", "failed to create directory")
	ErrConfigFilePersist              = NewImageCustomizerError("Config:FilePersist", "failed to persist saved configs file")
	ErrConfigFileExists               = NewImageCustomizerError("Config:FileExists", "failed to check if file exists")
	ErrConfigFileLoad                 = NewImageCustomizerError("Config:FileLoad", "failed to load saved configs file")
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

type LiveOSSavedConfigs struct {
	KdumpBootFiles    *imagecustomizerapi.KdumpBootFilesType `yaml:"kdumpBootFiles"`
	KernelCommandLine imagecustomizerapi.KernelCommandLine   `yaml:"kernelCommandLine"`
}

func (i *LiveOSSavedConfigs) IsValid() error {
	if i.KdumpBootFiles != nil {
		err := i.KdumpBootFiles.IsValid()
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrConfigInvalidKdumpBootFiles, err)
		}
	}

	err := i.KernelCommandLine.IsValid()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConfigInvalidKernelCommandLine, err)
	}

	return nil
}

type PxeSavedConfigs struct {
	bootstrapBaseUrl string `yaml:"bootstrapBaseUrl"`
	bootstrapFileUrl string `yaml:"bootstrapFileUrl"`
}

func (p *PxeSavedConfigs) IsValid() error {
	if p.bootstrapBaseUrl != "" && p.bootstrapFileUrl != "" {
		return ErrConfigBootstrapUrl
	}
	err := imagecustomizerapi.IsValidPxeUrl(p.bootstrapBaseUrl)
	if err != nil {
		return err
	}
	err = imagecustomizerapi.IsValidPxeUrl(p.bootstrapFileUrl)
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
	LiveOS LiveOSSavedConfigs `yaml:"liveos"`
	Pxe    PxeSavedConfigs    `yaml:"pxe"`
	OS     OSSavedConfigs     `yaml:"os"`
}

func (c *SavedConfigs) IsValid() (err error) {
	err = c.LiveOS.IsValid()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConfigInvalidIsoField, err)
	}

	err = c.Pxe.IsValid()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConfigInvalidPxeField, err)
	}

	err = c.OS.IsValid()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrConfigInvalidOsField, err)
	}

	return nil
}

func (c *SavedConfigs) persistSavedConfigs(savedConfigsFilePath string) (err error) {
	err = os.MkdirAll(filepath.Dir(savedConfigsFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrConfigDirectoryCreate, savedConfigsFilePath, err)
	}

	err = imagecustomizerapi.MarshalYamlFile(savedConfigsFilePath, c)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrConfigFilePersist, savedConfigsFilePath, err)
	}

	return nil
}

func loadSavedConfigs(savedConfigsFilePath string) (savedConfigs *SavedConfigs, err error) {
	exists, err := file.PathExists(savedConfigsFilePath)
	if err != nil {
		return nil, fmt.Errorf("%w (path='%s'):\n%w", ErrConfigFileExists, savedConfigsFilePath, err)
	}

	if !exists {
		return nil, nil
	}

	savedConfigs = &SavedConfigs{}
	err = imagecustomizerapi.UnmarshalAndValidateYamlFile(savedConfigsFilePath, savedConfigs)
	if err != nil {
		return nil, fmt.Errorf("%w (path='%s'):\n%w", ErrConfigFileLoad, savedConfigsFilePath, err)
	}

	return savedConfigs, nil
}

func updateSavedConfigs(savedConfigsFilePath string,
	newKdumpBootFiles *imagecustomizerapi.KdumpBootFilesType, newKernelCommandLine imagecustomizerapi.KernelCommandLine,
	newBootstrapBaseUrl string, newBootstrapFileUrl string, newDracutPackageInfo *PackageVersionInformation,
	newRequestedSelinuxMode imagecustomizerapi.SELinuxMode, newSELinuxPackageInfo *PackageVersionInformation,
) (outputConfigs *SavedConfigs, err error) {
	logger.Log.Infof("Updating saved configurations")
	outputConfigs = &SavedConfigs{}
	outputConfigs.LiveOS.KdumpBootFiles = newKdumpBootFiles
	outputConfigs.LiveOS.KernelCommandLine = newKernelCommandLine
	outputConfigs.Pxe.bootstrapBaseUrl = newBootstrapBaseUrl
	outputConfigs.Pxe.bootstrapFileUrl = newBootstrapFileUrl
	outputConfigs.OS.DracutPackageInfo = newDracutPackageInfo
	outputConfigs.OS.RequestedSELinuxMode = newRequestedSelinuxMode
	outputConfigs.OS.SELinuxPolicyPackageInfo = newSELinuxPackageInfo

	inputConfigs, err := loadSavedConfigs(savedConfigsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load saved configurations (%s):\n%w", savedConfigsFilePath, err)
	}

	if inputConfigs != nil {
		// if the kdumpBootFiles are not set, set it to the value from the previous run.
		if newKdumpBootFiles == nil {
			outputConfigs.LiveOS.KdumpBootFiles = inputConfigs.LiveOS.KdumpBootFiles
		}
		// do we have kernel arguments from a previous run?
		if len(inputConfigs.LiveOS.KernelCommandLine.ExtraCommandLine) > 0 {
			// If yes, add them before the new kernel arguments.
			savedArgs := inputConfigs.LiveOS.KernelCommandLine.ExtraCommandLine
			newArgs := newKernelCommandLine.ExtraCommandLine

			// Combine saved arguments with new ones
			combinedArgs := append(savedArgs, newArgs...)
			outputConfigs.LiveOS.KernelCommandLine.ExtraCommandLine = combinedArgs
		}

		// if the PXE iso image url is not set, set it to the value from the previous run.
		if newBootstrapBaseUrl == "" && inputConfigs.Pxe.bootstrapBaseUrl != "" {
			outputConfigs.Pxe.bootstrapBaseUrl = inputConfigs.Pxe.bootstrapBaseUrl
		}

		if newBootstrapFileUrl == "" && inputConfigs.Pxe.bootstrapFileUrl != "" {
			outputConfigs.Pxe.bootstrapFileUrl = inputConfigs.Pxe.bootstrapFileUrl
		}

		// if bootstrapBaseUrl is being set in this run (i.e. newBootstrapBaseUrl != ""),
		// then make sure bootstrapFileUrl is unset (since both fields must be mutually
		// exclusive) - and vice versa.
		if newBootstrapBaseUrl != "" {
			outputConfigs.Pxe.bootstrapFileUrl = ""
		}

		if newBootstrapFileUrl != "" {
			outputConfigs.Pxe.bootstrapBaseUrl = ""
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
