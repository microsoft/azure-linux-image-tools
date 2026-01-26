// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
)

const (
	buildTimeFormat = "2006-01-02T15:04:05Z"
)

var (
	ErrUkiKernelModified = NewImageCustomizerError("UKI:KernelModified",
		"kernel binaries detected in /boot after package operations. "+
			"Both 'passthrough' and 'modify' modes preserve the existing kernel and initramfs. "+
			"Use 'mode: create' to regenerate UKIs with updated kernels")
)

func doOsCustomizations(ctx context.Context, rc *ResolvedConfig, imageConnection *imageconnection.ImageConnection,
	partitionsCustomized bool, partitionsLayout []fstabEntryPartNum, distroHandler DistroHandler,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	// If UKI mode is 'create' and base image has UKIs, extract kernel and
	// initramfs from existing UKIs for re-customization. For 'passthrough'
	// mode, we skip extraction to preserve existing UKIs.
	if rc.Config.OS.Uki != nil && rc.Config.OS.Uki.Mode == imagecustomizerapi.UkiModeCreate {
		// Check if base image has UKIs to determine if extraction is needed
		hasUkis, err := baseImageHasUkis(imageChroot)
		if err != nil {
			return err
		}

		if hasUkis {
			// Base image has UKIs and mode is create - extract for re-customization
			err = extractKernelAndInitramfsFromUkis(ctx, imageChroot, rc.BuildDirAbs)
			if err != nil {
				return err
			}
		}
	}

	// If UKI mode is 'modify', extract cmdline early so BootCustomizer can modify it
	if rc.Config.OS.Uki != nil && rc.Config.OS.Uki.Mode == imagecustomizerapi.UkiModeModify {
		ukiBuildDir := filepath.Join(rc.BuildDirAbs, UkiBuildDir)
		err = os.MkdirAll(ukiBuildDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create UKI build directory:\n%w", err)
		}

		err = extractAndSaveUkiCmdline(rc.BuildDirAbs, imageChroot)
		if err != nil {
			return fmt.Errorf("failed to extract UKI cmdline for modify mode:\n%w", err)
		}
	}

	for _, configWithBase := range rc.ConfigChain {
		snapshotTime := configWithBase.Config.OS.Packages.SnapshotTime
		if rc.Options.PackageSnapshotTime != "" {
			snapshotTime = rc.Options.PackageSnapshotTime
		}

		err = addRemoveAndUpdatePackages(ctx, rc.BuildDirAbs, rc.BaseConfigPath, configWithBase.Config.OS,
			imageChroot, nil, rc.Options.RpmsSources, rc.Options.UseBaseImageRpmRepos, distroHandler,
			snapshotTime)
		if err != nil {
			return err
		}
	}

	// Both modes preserve the existing kernel and initramfs:
	// - passthrough: preserves entire UKI without modification
	// - modify: preserves main UKI (kernel, initramfs) and only modifies addon
	if rc.Config.OS.Uki != nil && (rc.Config.OS.Uki.Mode == imagecustomizerapi.UkiModePassthrough ||
		rc.Config.OS.Uki.Mode == imagecustomizerapi.UkiModeModify) {
		hasKernels, err := hasKernelBinariesInBoot(imageChroot.RootDir())
		if err != nil {
			return err
		}
		if hasKernels {
			return ErrUkiKernelModified
		}
	}

	err = UpdateHostname(ctx, rc.Config.OS.Hostname, imageChroot)
	if err != nil {
		return err
	}

	for _, configWithBase := range rc.ConfigChain {
		err = AddOrUpdateGroups(ctx, configWithBase.Config.OS.Groups, imageChroot)
		if err != nil {
			return err
		}

		err = AddOrUpdateUsers(ctx, configWithBase.Config.OS.Users, configWithBase.BaseConfigPath, imageChroot)
		if err != nil {
			return err
		}
	}

	for _, configWithBase := range rc.ConfigChain {
		err = copyAdditionalDirs(ctx, configWithBase.BaseConfigPath, configWithBase.Config.OS.AdditionalDirs, imageChroot)
		if err != nil {
			return err
		}
	}

	for _, configWithBase := range rc.ConfigChain {
		err = copyAdditionalFiles(ctx, configWithBase.BaseConfigPath, configWithBase.Config.OS.AdditionalFiles,
			imageChroot)
		if err != nil {
			return err
		}
	}

	for _, configWithBase := range rc.ConfigChain {
		err = EnableOrDisableServices(ctx, configWithBase.Config.OS.Services, imageChroot)
		if err != nil {
			return err
		}
	}

	for _, configWithBase := range rc.ConfigChain {
		err = LoadOrDisableModules(ctx, configWithBase.Config.OS.Modules, imageChroot.RootDir())
		if err != nil {
			return err
		}
	}

	err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, rc.ImageUuidStr)
	if err != nil {
		return err
	}

	if rc.Config.OS.ImageHistory != imagecustomizerapi.ImageHistoryNone {
		err = addImageHistory(ctx, imageChroot, rc.ImageUuidStr, rc.BaseConfigPath, ToolVersion, buildTime, rc.Config)
		if err != nil {
			return err
		}
	}

	err = handleBootLoader(ctx, rc, imageConnection, partitionsLayout, false, distroHandler)
	if err != nil {
		return err
	}

	selinuxMode, err := handleSELinux(ctx, rc.BuildDirAbs, rc.SELinux.Mode, rc.BootLoader.ResetType,
		imageChroot, rc.Uki, distroHandler)
	if err != nil {
		return err
	}

	var overlayUpdated bool
	for _, configWithBase := range rc.ConfigChain {
		updated, err := enableOverlays(ctx, configWithBase.Config.OS.Overlays, selinuxMode, imageChroot)
		if err != nil {
			return err
		}
		overlayUpdated = overlayUpdated || updated
	}

	verityUpdated, err := enableVerityPartition(ctx, rc.BuildDirAbs, rc.Config.Storage.Verity, imageChroot, distroHandler, rc.Uki)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		err = regenerateInitrd(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = prepareUki(ctx, rc.BuildDirAbs, rc.Config.OS.Uki, imageChroot, distroHandler)
	if err != nil {
		return err
	}

	err = restoreResolvConf(ctx, resolvConf, imageChroot)
	if err != nil {
		return err
	}

	err = selinuxSetFiles(ctx, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.FinalizeCustomization, "finalizeCustomization",
		imageChroot)
	if err != nil {
		return err
	}

	err = checkForInstalledKernel(ctx, imageChroot)
	if err != nil {
		return err
	}

	return nil
}

func hasKernelBinariesInBoot(rootDir string) (bool, error) {
	bootDir := filepath.Join(rootDir, "boot")
	entries, err := os.ReadDir(bootDir)
	if err != nil {
		if os.IsNotExist(err) {
			// /boot doesn't exist, no kernels
			return false, nil
		}
		return false, fmt.Errorf("failed to read /boot directory:\n%w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Check for kernel binaries: vmlinuz-* (e.g., vmlinuz-6.6.104.2-4.azl3)
		if strings.HasPrefix(entry.Name(), "vmlinuz-") {
			return true, nil
		}
		// Check for initramfs files: initramfs-*.img (e.g., initramfs-6.6.104.2-4.azl3.img)
		if strings.HasPrefix(entry.Name(), "initramfs-") {
			return true, nil
		}
	}

	return false, nil
}
