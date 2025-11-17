// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

var (
	// Boot customization errors
	ErrBootGrubMkconfigGeneration = NewImageCustomizerError("Boot:GrubMkconfigGeneration", "failed to generate grub.cfg via grub2-mkconfig")
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
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	defaultGrubFileContent, err := readDefaultGrubFile(imageChroot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
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
		// If grub config file is missing, indicate that with fs.ErrNotExist.
		if b.defaultGrubFileContent == "" {
			return imagecustomizerapi.SELinuxModeDefault, fs.ErrNotExist
		}

		// Check both GRUB_CMDLINE_LINUX and GRUB_CMDLINE_LINUX_DEFAULT variables
		// and merge arguments from both if they exist
		args, err = GetDefaultGrubFileLinuxArgsFromMultipleVars(b.defaultGrubFileContent)
		if err != nil {
			return "", fmt.Errorf("failed to find SELinux args in grub file (%s):\n%w", installutils.GrubDefFile, err)
		}
	} else {
		// Same here, if grub config file is missing, indicate that with fs.ErrNotExist.
		if b.grubCfgContent == "" {
			return imagecustomizerapi.SELinuxModeDefault, fs.ErrNotExist
		}

		args, _, err = getLinuxCommandLineArgs(b.grubCfgContent)
		if err != nil {
			return imagecustomizerapi.SELinuxModeDefault,
				fmt.Errorf("failed to parse SELinux args from grub file (%s):\n%w", installutils.GrubCfgFile, err)
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
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return imagecustomizerapi.SELinuxModeDefault, err
	} else if !errors.Is(err, fs.ErrNotExist) && selinuxMode != imagecustomizerapi.SELinuxModeDefault {
		return selinuxMode, nil
	}

	// Fallback to extracting from UKI if grub.cfg doesn't exist or returns default.
	selinuxMode, err = b.getSELinuxModeFromUki(imageChroot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("failed to extract SELinux mode from UKI:\n%w", err)
	} else if !errors.Is(err, fs.ErrNotExist) && selinuxMode != imagecustomizerapi.SELinuxModeDefault {
		return selinuxMode, nil
	}

	// Final fallback: Get the SELinux mode from the /etc/selinux/config file.
	selinuxMode, err = getSELinuxModeFromConfigFile(imageChroot)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	return selinuxMode, nil
}

// Extract SELinux mode from UKI kernel command-line arguments.
func (b *BootCustomizer) getSELinuxModeFromUki(imageChroot safechroot.ChrootInterface) (imagecustomizerapi.SELinuxMode, error) {
	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	buildDir := filepath.Join(filepath.Dir(imageChroot.RootDir()), "uki-selinux-temp")

	err := os.MkdirAll(buildDir, 0o755)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	kernelToArgs, err := extractKernelCmdlineFromUkiEfis(espDir, buildDir)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
	}

	if len(kernelToArgs) == 0 {
		return imagecustomizerapi.SELinuxModeDefault, fs.ErrNotExist
	}

	// Use the first kernel's cmdline since they should all have the same SELinux settings.
	var firstCmdline string
	for _, cmdline := range kernelToArgs {
		firstCmdline = cmdline
		break
	}

	logger.Log.Debugf("Extracting SELinux mode from UKI cmdline: %s", firstCmdline)

	// Parse the cmdline string into grub tokens, then into args
	tokens, err := grub.TokenizeConfig(firstCmdline)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("failed to tokenize cmdline from UKI: %w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, fmt.Errorf("failed to parse cmdline args from UKI: %w", err)
	}

	// Extract SELinux mode from the parsed arguments
	selinuxMode, err := getSELinuxModeFromLinuxArgs(args)
	if err != nil {
		return imagecustomizerapi.SELinuxModeDefault, err
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
			return fmt.Errorf("%w:\n%w", ErrBootGrubMkconfigGeneration, err)
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
