// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

func CustomizeImageHelperImageCreator(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	rawImageFile string, rpmsSources []string, useBaseImageRpmRepos bool,
	imageUuidStr string, packageSnapshotTime string, tarFile string,
) (map[string]diskutils.FstabEntry, string, error) {
	logger.Log.Debugf("Customizing OS image with config file %s", baseConfigPath)

	toolsChrootDir := filepath.Join(buildDir, toolsRoot)
	toolsChroot := safechroot.NewChroot(toolsChrootDir, false)
	err := toolsChroot.Initialize(tarFile, nil, nil, true)
	if err != nil {
		return nil, "", err
	}
	defer toolsChroot.Close(false)

	imageConnection, partUuidToFstabEntry, _, _, err := connectToExistingImage(ctx, rawImageFile, toolsChrootDir,
		toolsRootImageDir, true, false, false, false)
	if err != nil {
		return nil, "", err
	}
	defer imageConnection.Close()

	// Do the actual customizations.
	err = doOsCustomizationsImageCreator(ctx, buildDir, baseConfigPath, config, imageConnection, toolsChroot, rpmsSources,
		useBaseImageRpmRepos, imageUuidStr,
		partUuidToFstabEntry, packageSnapshotTime)

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(buildDir, imageConnection)
	if err != nil {
		return nil, "", err
	}

	// Extract OS release info from rootfs for COSI
	osRelease, err := extractOSRelease(imageConnection)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract OS release from rootfs partition:\n%w", err)
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, "", err
	}

	// Close the tools chroot and image connection.
	err = toolsChroot.Close(false)
	if err != nil {
		return nil, "", err
	}

	return partUuidToFstabEntry, osRelease, nil
}

func doOsCustomizationsImageCreator(
	ctx context.Context,
	buildDir string, baseConfigPath string,
	config *imagecustomizerapi.Config,
	imageConnection *imageconnection.ImageConnection,
	toolsChroot *safechroot.Chroot,
	rpmsSources []string,
	useBaseImageRpmRepos bool,
	imageUuid string,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
	packageSnapshotTime string,
) error {
	imageChroot := imageConnection.Chroot()
	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(toolsChroot)
	if err != nil {
		return err
	}

	if err = addRemoveAndUpdatePackages(
		ctx,
		buildDir, baseConfigPath, config.OS, imageChroot, toolsChroot, rpmsSources,
		useBaseImageRpmRepos, packageSnapshotTime); err != nil {
		return err
	}

	if err = UpdateHostname(ctx, config.OS.Hostname, imageChroot); err != nil {
		return err
	}

	if err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, imageUuid); err != nil {
		return err
	}

	if err = handleBootLoader(ctx, baseConfigPath, config, imageConnection, partUuidToFstabEntry, true); err != nil {
		return err
	}

	// Clear systemd state files that should be unique to each instance
	err = clearSystemdState(imageChroot)
	if err != nil {
		return fmt.Errorf("failed to clear systemd state:\n%w", err)
	}

	err = runUserScripts(ctx, baseConfigPath, config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	if err = restoreResolvConf(ctx, resolvConf, imageChroot); err != nil {
		return err
	}

	if err = checkForInstalledKernel(ctx, imageChroot); err != nil {
		return err
	}

	return nil
}

// clearSystemdState clears the systemd state files that should be unique to each instance of the image.
// This is based on https://systemd.io/BUILDING_IMAGES/. Primarily, this function will ensure that
// /etc/machine-id is configured correctly, and that random seed and credential files are removed if they exist.
// For Image Creator, we disable systemd firstboot by default (set machine-id to empty) since Azure Linux
// has traditionally not used firstboot mechanisms.
func clearSystemdState(imageChroot *safechroot.Chroot) error {
	const (
		machineIDFile      = "/etc/machine-id"
		machineIDContent   = "" // Empty for disabled firstboot
		machineIDFilePerms = 0o444
	)

	// These state files are very unlikely to be present, but we should be thorough and check for them.
	// See https://systemd.io/BUILDING_IMAGES/ for more information.
	otherFilesToRemove := []string{
		"/var/lib/systemd/random-seed",
		"/boot/efi/loader/random-seed",
		"/var/lib/systemd/credential.secret",
	}

	logger.Log.Debug("Configuring systemd state files")

	// The systemd package will create this file, but if it's not installed, we need to create it.
	machineIDPath := filepath.Join(imageChroot.RootDir(), machineIDFile)
	exists, err := file.PathExists(machineIDPath)
	if err != nil {
		return fmt.Errorf("failed to check if machine-id exists: %w", err)
	}

	if !exists {
		logger.Log.Debug("Creating empty machine-id file")
		err = file.Create(machineIDPath, machineIDFilePerms)
		if err != nil {
			return fmt.Errorf("failed to create empty machine-id: %w", err)
		}
	}

	// Set machine-id to empty (disables systemd firstboot)
	logger.Log.Debug("Disabling systemd firstboot")
	err = file.Write(machineIDContent, machineIDPath)
	if err != nil {
		return fmt.Errorf("failed to write empty machine-id: %w", err)
	}

	// These files should not be present in the image, but per https://systemd.io/BUILDING_IMAGES/ we should
	// be thorough and double-check.
	for _, filePath := range otherFilesToRemove {
		fullPath := filepath.Join(imageChroot.RootDir(), filePath)
		exists, err = file.PathExists(fullPath)
		if err != nil {
			return fmt.Errorf("failed to check if systemd state file (%s) exists: %w", filePath, err)
		}

		// Do an explicit check for existence so we can log the file removal.
		if exists {
			logger.Log.Debugf("Removing systemd state file (%s)", filePath)
			err = file.RemoveFileIfExists(fullPath)
			if err != nil {
				return fmt.Errorf("failed to remove systemd state file (%s): %w", filePath, err)
			}
		}
	}

	return nil
}
