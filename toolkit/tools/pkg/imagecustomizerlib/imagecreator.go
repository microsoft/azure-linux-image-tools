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

	imageConnection, partUuidToFstabEntry, _, err := connectToExistingImage(ctx, rawImageFile, toolsChrootDir, toolsRootImageDir, true, false)
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

	err = runUserScripts(ctx, baseConfigPath, config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot)
	if err != nil {
		return err
	}

	return nil
}
