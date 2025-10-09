// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

func CustomizeImageHelperImageCreator(ctx context.Context, rc *ResolvedConfig, tarFile string,
	distroHandler distroHandler,
) (map[string]diskutils.FstabEntry, string, error) {
	logger.Log.Debugf("Customizing OS image with config file %s", rc.BaseConfigPath)

	toolsChrootDir := filepath.Join(rc.BuildDirAbs, toolsRoot)
	toolsChroot := safechroot.NewChroot(toolsChrootDir, false)
	err := toolsChroot.Initialize(tarFile, nil, nil, true)
	if err != nil {
		return nil, "", err
	}
	defer toolsChroot.Close(false)

	imageConnection, partUuidToFstabEntry, _, _, err := connectToExistingImage(ctx, rc.RawImageFile, toolsChrootDir,
		toolsRootImageDir, true, false, false, false)
	if err != nil {
		return nil, "", err
	}
	defer imageConnection.Close()

	// Do the actual customizations.
	err = doOsCustomizationsImageCreator(ctx, rc, imageConnection, toolsChroot, partUuidToFstabEntry, distroHandler)

	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.
	warnOnLowFreeSpace(rc.BuildDirAbs, imageConnection)
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
	rc *ResolvedConfig,
	imageConnection *imageconnection.ImageConnection,
	toolsChroot *safechroot.Chroot,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
	distroHandler distroHandler,
) error {
	imageChroot := imageConnection.Chroot()
	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(toolsChroot)
	if err != nil {
		return err
	}

	if err = addRemoveAndUpdatePackages(
		ctx,
		rc.BuildDirAbs, rc.BaseConfigPath, rc.Config.OS, imageChroot, toolsChroot, rc.Options.RpmsSources,
		rc.Options.UseBaseImageRpmRepos, distroHandler, rc.PackageSnapshotTime); err != nil {
		return err
	}

	if err = UpdateHostname(ctx, rc.Config.OS.Hostname, imageChroot); err != nil {
		return err
	}

	if err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, rc.ImageUuidStr); err != nil {
		return err
	}

	if err = handleBootLoader(ctx, rc.BaseConfigPath, rc.Config, imageConnection, partUuidToFstabEntry, true); err != nil {
		return err
	}

	// Clear systemd state files that should be unique to each instance
	// For Image Creator, we disable systemd firstboot by default since Azure Linux
	// has traditionally not used firstboot mechanisms.
	err = installutils.ClearSystemdState(imageChroot, false)
	if err != nil {
		return fmt.Errorf("failed to clear systemd state:\n%w", err)
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	if err = restoreResolvConf(ctx, resolvConf, imageChroot); err != nil {
		return err
	}

	if err = checkForInstalledKernel(ctx, imageChroot); err != nil {
		return err
	}

	err = runUserScripts(ctx, rc.BaseConfigPath, rc.Config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot)
	if err != nil {
		return err
	}

	return nil
}
