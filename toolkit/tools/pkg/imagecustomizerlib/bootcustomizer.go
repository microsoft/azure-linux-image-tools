// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

type BootCustomizer struct {
	// The contents of the /boot/grub2/grub.cfg file.
	grubCfgContent string

	// The contents of the /etc/default/grub file.
	defaultGrubFileContent string

	// Whether or not the image is using grub-mkconfig.
	isGrubMkconfig bool
}

func NewBootCustomizer(imageChroot safechroot.ChrootInterface) (*BootCustomizer, error) {
	grubCfgContent, err := ReadGrub2ConfigFile(imageChroot)
	if err != nil {
		return nil, err
	}

	defaultGrubFileContent, err := readDefaultGrubFile(imageChroot)
	if err != nil {
		return nil, err
	}

	isGrubMkconfig := isGrubMkconfigConfig(grubCfgContent)

	b := &BootCustomizer{
		grubCfgContent:         grubCfgContent,
		defaultGrubFileContent: defaultGrubFileContent,
		isGrubMkconfig:         isGrubMkconfig,
	}
	return b, nil
}

// Returns whether or not the OS uses grub-mkconfig.
func (b *BootCustomizer) IsGrubMkconfigImage() bool {
	return b.isGrubMkconfig
}

// Inserts new kernel command-line args into the grub config file.
func (b *BootCustomizer) AddKernelCommandLine(extraCommandLine []string) error {
	if len(extraCommandLine) <= 0 {
		return nil
	}

	combinedArgs := GrubArgsToString(extraCommandLine)

	if b.isGrubMkconfig {
		defaultGrubFileContent, err := addExtraCommandLineToDefaultGrubFile(b.defaultGrubFileContent, combinedArgs)
		if err != nil {
			return err
		}

		b.defaultGrubFileContent = defaultGrubFileContent
	} else {
		// Add the args directly to the /boot/grub2/grub.cfg file.
		grubCfgContent, err := appendKernelCommandLineArgsAll(b.grubCfgContent, combinedArgs)
		if err != nil {
			return err
		}

		b.grubCfgContent = grubCfgContent
	}

	return nil
}

// Gets the image's configured SELinux mode.
func (b *BootCustomizer) getSELinuxModeFromGrub() (imagecustomizerapi.SELinuxMode, error) {
	var err error
	var args []grubConfigLinuxArg

	// Get the SELinux kernel command-line args.
	if b.isGrubMkconfig {
		_, args, _, err = GetDefaultGrubFileLinuxArgs(b.defaultGrubFileContent, defaultGrubFileVarNameCmdlineForSELinux)
		if err != nil {
			return "", err
		}
	} else {
		args, _, err = getLinuxCommandLineArgs(b.grubCfgContent)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault, err
		}
	}

	// Get the SELinux mode from the kernel command-line args.
	selinuxMode, err := getSELinuxModeFromLinuxArgs(args)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	return selinuxMode, nil
}

func (b *BootCustomizer) GetSELinuxMode(imageChroot safechroot.ChrootInterface) (imagecustomizerapi.SELinuxMode, error) {
	// Get the SELinux mode from the kernel command-line args.
	selinuxMode, err := b.getSELinuxModeFromGrub()
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	if selinuxMode == imagecustomizerapi.SELinuxModeDefault {
		// Get the SELinux mode from the /etc/selinux/config file.
		selinuxMode, err = getSELinuxModeFromConfigFile(imageChroot)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault, err
		}
	}

	return selinuxMode, nil
}

// Update the image's SELinux kernel command-line args.
func (b *BootCustomizer) UpdateSELinuxCommandLine(selinuxMode imagecustomizerapi.SELinuxMode) error {
	newSELinuxArgs, err := selinuxModeToArgs(selinuxMode)
	if err != nil {
		return err
	}

	err = b.UpdateKernelCommandLineArgs(defaultGrubFileVarNameCmdlineForSELinux, selinuxArgNames, newSELinuxArgs)
	if err != nil {
		return err
	}

	return nil
}

// Update the image's SELinux kernel command-line args for OSModifier.
func (b *BootCustomizer) UpdateSELinuxCommandLineForEMU(selinuxMode imagecustomizerapi.SELinuxMode) error {
	newSELinuxArgs, err := selinuxModeToArgsWithPermissiveFlag(selinuxMode)
	if err != nil {
		return err
	}

	err = b.UpdateKernelCommandLineArgs(defaultGrubFileVarNameCmdlineForSELinux, selinuxArgNames, newSELinuxArgs)
	if err != nil {
		return err
	}

	return nil
}

func (b *BootCustomizer) UpdateKernelCommandLineArgs(defaultGrubFileVarName defaultGrubFileVarName,
	argsToRemove []string, newArgs []string,
) error {
	if b.isGrubMkconfig {
		defaultGrubFileContent, err := updateDefaultGrubFileKernelCommandLineArgs(b.defaultGrubFileContent,
			defaultGrubFileVarName, argsToRemove, newArgs)
		if err != nil {
			return err
		}

		b.defaultGrubFileContent = defaultGrubFileContent
	} else {
		grubCfgContent, err := updateKernelCommandLineArgsAll(b.grubCfgContent, argsToRemove, newArgs)
		if err != nil {
			return err
		}

		b.grubCfgContent = grubCfgContent
	}

	return nil
}

// Makes changes to the /etc/default/grub file that are needed/useful for enabling verity.
func (b *BootCustomizer) PrepareForVerity() error {
	if b.isGrubMkconfig {
		// Force root command-line arg to be referenced by /dev path instead of by UUID.
		defaultGrubFileContent, err := UpdateDefaultGrubFileVariable(b.defaultGrubFileContent, "GRUB_DISABLE_UUID",
			"true")
		if err != nil {
			return err
		}

		// For rootfs verity, the root device will always be "/dev/mapper/root"
		rootDevicePath := imagecustomizerapi.VerityRootDevicePath
		defaultGrubFileContent, err = UpdateDefaultGrubFileVariable(defaultGrubFileContent, "GRUB_DEVICE",
			rootDevicePath)
		if err != nil {
			return err
		}

		b.defaultGrubFileContent = defaultGrubFileContent
	}

	return nil
}

func (b *BootCustomizer) WriteToFile(imageChroot safechroot.ChrootInterface) error {
	if b.isGrubMkconfig {
		// Update /etc/defaukt/grub file.
		err := WriteDefaultGrubFile(b.defaultGrubFileContent, imageChroot)
		if err != nil {
			return err
		}
		// Update /boot/grub2/grub.cfg file.
		err = installutils.CallGrubMkconfig(imageChroot)
		if err != nil {
			return fmt.Errorf("failed to generate grub.cfg via grub2-mkconfig:\n%w", err)
		}
	} else {
		// Update grub.cfg file.
		err := writeGrub2ConfigFile(b.grubCfgContent, imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *BootCustomizer) SetRootDevice(rootDevice string) error {
	updatedGrubFileContent, err := UpdateDefaultGrubFileVariable(b.defaultGrubFileContent, "GRUB_DEVICE", rootDevice)
	if err != nil {
		return err
	}

	b.defaultGrubFileContent = updatedGrubFileContent

	return nil
}
