// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierlib

import (
	"fmt"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/osmodifierapi"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

func doModifications(baseConfigPath string, osConfig *osmodifierapi.OS) error {
	var dummyChroot safechroot.ChrootInterface = &safechroot.DummyChroot{}

	err := imagecustomizerlib.AddOrUpdateUsers(osConfig.Users, baseConfigPath, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.UpdateHostname(osConfig.Hostname, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.EnableOrDisableServices(osConfig.Services, dummyChroot)
	if err != nil {
		return err
	}

	err = imagecustomizerlib.LoadOrDisableModules(osConfig.Modules, dummyChroot.RootDir())
	if err != nil {
		return err
	}

	// Only initialize BootCustomizer if any GRUB-related settings are present
	needsBootCustomizer := osConfig.KernelCommandLine.ExtraCommandLine != nil ||
		osConfig.Overlays != nil ||
		osConfig.SELinux.Mode != "" ||
		osConfig.Verity != nil ||
		osConfig.RootDevice != ""

	var bootCustomizer *imagecustomizerlib.BootCustomizer
	if needsBootCustomizer {
		bootCustomizer, err = imagecustomizerlib.NewBootCustomizer(dummyChroot)
		if err != nil {
			return err
		}
	}

	updateGrubRequired := false

	if osConfig.KernelCommandLine.ExtraCommandLine != nil {
		err = bootCustomizer.AddKernelCommandLine(osConfig.KernelCommandLine.ExtraCommandLine)
		if err != nil {
			return fmt.Errorf("failed to add extra kernel command line:\n%w", err)
		}
		updateGrubRequired = true
	}

	if osConfig.Overlays != nil {
		err = updateGrubConfigForOverlay(*osConfig.Overlays, bootCustomizer)
		if err != nil {
			return err
		}
		updateGrubRequired = true
	}

	if osConfig.SELinux.Mode != "" {
		err = handleSELinux(osConfig.SELinux.Mode, bootCustomizer, dummyChroot)
		if err != nil {
			return err
		}
		updateGrubRequired = true
	}

	if osConfig.Verity != nil {
		err = updateDefaultGrubForVerity(osConfig.Verity, bootCustomizer)
		if err != nil {
			return err
		}
		updateGrubRequired = true
	}

	if osConfig.RootDevice != "" {
		err = bootCustomizer.SetRootDevice(osConfig.RootDevice)
		if err != nil {
			return err
		}
		updateGrubRequired = true
	}

	// Write changes to file only if GRUB needs updating
	if updateGrubRequired {
		err = bootCustomizer.WriteToFile(dummyChroot)
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

func handleSELinux(selinuxMode imagecustomizerapi.SELinuxMode, bootCustomizer *imagecustomizerlib.BootCustomizer, dummyChroot safechroot.ChrootInterface) error {
	var err error
	currentSELinuxMode, err := bootCustomizer.GetSELinuxMode(dummyChroot)
	if err != nil {
		return fmt.Errorf("failed to get current SELinux mode:\n%w", err)
	}

	if selinuxMode == imagecustomizerapi.SELinuxModeDefault || selinuxMode == currentSELinuxMode {
		// Don't need to change the configured SELinux mode.
		return nil
	}

	logger.Log.Infof("Configuring SELinux mode")

	err = bootCustomizer.UpdateSELinuxCommandLineForEMU(selinuxMode)
	if err != nil {
		return err
	}

	// No need to set SELinux labels here as in trident there is reset labels at the end
	return nil
}
