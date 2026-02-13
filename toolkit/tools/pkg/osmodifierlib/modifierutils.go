// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierlib

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/osmodifierapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/pkg/imagecustomizerlib"
)

func doModifications(ctx context.Context, baseConfigPath string, osConfig *osmodifierapi.OS) error {
	var dummyChroot safechroot.ChrootInterface = &safechroot.DummyChroot{}

	// Create a temporary directory for operations that need a build directory
	buildDir, err := os.MkdirTemp("", "osmodifier-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary build directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	distroHandler, err := imagecustomizerlib.NewDistroHandlerFromChroot(dummyChroot)
	if err != nil {
		return err
	}

	if osConfig.SELinux.Mode != imagecustomizerapi.SELinuxModeDefault && !distroHandler.SELinuxSupported() {
		return fmt.Errorf("%w: cannot set SELinux mode (%s)", imagecustomizerlib.ErrSELinuxNotSupported,
			osConfig.SELinux.Mode)
	}

	err = imagecustomizerlib.AddOrUpdateUsers(ctx, osConfig.Users, baseConfigPath, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.UpdateHostname(ctx, osConfig.Hostname, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.EnableOrDisableServices(ctx, osConfig.Services, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.LoadOrDisableModules(ctx, osConfig.Modules, dummyChroot.RootDir())
	if err != nil {
		return err
	}

	// Add a check to make sure BootCustomizer can be initialized
	bootloaderType, err := distroHandler.DetectBootloaderType(dummyChroot)
	if err != nil {
		return err
	}

	needsBootCustomizer := bootloaderType == imagecustomizerlib.BootloaderTypeGrub &&
		(osConfig.KernelCommandLine.ExtraCommandLine != nil ||
			osConfig.Overlays != nil ||
			osConfig.SELinux.Mode != imagecustomizerapi.SELinuxModeDefault ||
			osConfig.Verity != nil ||
			osConfig.RootDevice != "")

	var bootCustomizer *imagecustomizerlib.BootCustomizer
	if needsBootCustomizer {
		bootCustomizer, err = imagecustomizerlib.NewBootCustomizer(dummyChroot, nil, buildDir, distroHandler)
		if err != nil {
			return err
		}

		if osConfig.KernelCommandLine.ExtraCommandLine != nil {
			err = bootCustomizer.AddKernelCommandLine(osConfig.KernelCommandLine.ExtraCommandLine)
			if err != nil {
				return fmt.Errorf("failed to add extra kernel command line:\n%w", err)
			}
		}

		if osConfig.Overlays != nil {
			err = updateGrubConfigForOverlay(*osConfig.Overlays, bootCustomizer)
			if err != nil {
				return err
			}
		}

		if osConfig.Verity != nil {
			err = updateDefaultGrubForVerity(osConfig.Verity, bootCustomizer)
			if err != nil {
				return err
			}
		}

		if osConfig.RootDevice != "" {
			err = bootCustomizer.SetRootDevice(osConfig.RootDevice)
			if err != nil {
				return err
			}
		}

		if osConfig.SELinux.Mode != imagecustomizerapi.SELinuxModeDefault {
			err = updateSELinuxForGrubBasedBoot(buildDir, osConfig.SELinux.Mode, bootCustomizer, dummyChroot)
			if err != nil {
				return err
			}
		}

		err = bootCustomizer.WriteToFile(dummyChroot)
		if err != nil {
			return err
		}
	}

	if osConfig.SELinux.Mode != imagecustomizerapi.SELinuxModeDefault &&
		bootloaderType == imagecustomizerlib.BootloaderTypeSystemdBoot {
		err = updateSELinuxForUkiBoot(osConfig.SELinux.Mode, dummyChroot)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateDefaultGrubForVerity(verity *osmodifierapi.Verity, bootCustomizer *imagecustomizerlib.BootCustomizer) error {
	var err error

	formattedCorruptionOption, err := imagecustomizerlib.SystemdFormatCorruptionOption(verity.CorruptionOption)
	if err != nil {
		return err
	}

	newArgs := []string{
		"rd.systemd.verity=1",
		fmt.Sprintf("systemd.verity_root_data=%s", verity.DataDevice),
		fmt.Sprintf("systemd.verity_root_hash=%s", verity.HashDevice),
		fmt.Sprintf("systemd.verity_root_options=%s", formattedCorruptionOption),
	}

	err = bootCustomizer.UpdateKernelCommandLineArgs("GRUB_CMDLINE_LINUX", []string{"rd.systemd.verity",
		"systemd.verity_root_data", "systemd.verity_root_hash", "systemd.verity_root_options"}, newArgs)
	if err != nil {
		return err
	}

	return nil
}

func updateGrubConfigForOverlay(overlays []osmodifierapi.Overlay, bootCustomizer *imagecustomizerlib.BootCustomizer) error {
	var err error
	var overlayConfigs []string

	// Iterate over each Overlay configuration
	for _, overlay := range overlays {
		// Construct the argument for each Overlay
		overlayConfig := fmt.Sprintf(
			"%s,%s,%s,%s",
			overlay.LowerDir, overlay.UpperDir, overlay.WorkDir, overlay.Partition.Id,
		)
		overlayConfigs = append(overlayConfigs, overlayConfig)
	}

	// Concatenate all overlay configurations with spaces
	concatenatedOverlays := strings.Join(overlayConfigs, " ")

	// Construct the final cmdline argument
	newArgs := []string{
		fmt.Sprintf("rd.overlayfs=%s", concatenatedOverlays),
	}

	err = bootCustomizer.UpdateKernelCommandLineArgs("GRUB_CMDLINE_LINUX", []string{"rd.overlayfs"},
		newArgs)
	if err != nil {
		return err
	}

	return nil
}

func updateSELinuxForUkiBoot(selinuxMode imagecustomizerapi.SELinuxMode, installChroot safechroot.ChrootInterface) error {
	if selinuxMode == imagecustomizerapi.SELinuxModeDefault {
		return nil
	}

	logger.Log.Infof("Applying SELinux mode ('%s') for UKI-based system", selinuxMode)

	err := imagecustomizerlib.UpdateSELinuxModeInConfigFile(selinuxMode, installChroot)
	if err != nil {
		return fmt.Errorf("failed to update SELinux mode in config file: %w", err)
	}

	return nil
}

func updateSELinuxForGrubBasedBoot(buildDir string, selinuxMode imagecustomizerapi.SELinuxMode, bootCustomizer *imagecustomizerlib.BootCustomizer, installChroot safechroot.ChrootInterface) error {
	currentSELinuxMode, err := bootCustomizer.GetSELinuxMode(buildDir, installChroot)
	if err != nil {
		return fmt.Errorf("failed to get current SELinux mode: %w", err)
	}

	if selinuxMode == imagecustomizerapi.SELinuxModeDefault || selinuxMode == currentSELinuxMode {
		return nil
	}

	logger.Log.Infof("Updating SELinux mode from ('%s') to ('%s') for GRUB-based system", currentSELinuxMode, selinuxMode)

	err = bootCustomizer.UpdateSELinuxCommandLineForEMU(selinuxMode)
	if err != nil {
		return fmt.Errorf("failed to update SELinux kernel cmdline: %w", err)
	}

	err = imagecustomizerlib.UpdateSELinuxModeInConfigFile(selinuxMode, installChroot)
	if err != nil {
		return fmt.Errorf("failed to update SELinux mode in config file: %w", err)
	}

	return nil
}
