// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

func CustomizeImageHelperCreate(ctx context.Context, rc *ResolvedConfig, toolsDir string,
	distroHandler DistroHandler,
) ([]fstabEntryPartNum, string, error) {
	logger.Log.Debugf("Customizing OS image")

	toolsChroot, err := initToolsChroot(toolsDir)
	if err != nil {
		return nil, "", err
	}
	defer toolsChroot.Close(ctx)

	imageConnection, partitionsLayout, _, _, _, err := connectToExistingImage(ctx, rc.RawImageFile, toolsDir,
		toolsRootImageDir, true, false, false, false, distroHandler)
	if err != nil {
		return nil, "", err
	}
	defer imageConnection.Close()

	// Do the actual customizations.
	err = doOsCustomizationsCreate(ctx, rc, imageConnection, toolsChroot.Chroot(), partitionsLayout, distroHandler)

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

	err = toolsChroot.CleanClose(ctx)
	if err != nil {
		return nil, "", err
	}

	return partitionsLayout, osRelease, nil
}

func doOsCustomizationsCreate(
	ctx context.Context,
	rc *ResolvedConfig,
	imageConnection *imageconnection.ImageConnection,
	toolsChroot *safechroot.Chroot,
	partitionsLayout []fstabEntryPartNum,
	distroHandler DistroHandler,
) error {
	imageChroot := imageConnection.Chroot()
	buildTime := time.Now().Format(buildTimeFormat)

	// Override resolv.conf inside the image chroot so user scripts and RPM
	// scriptlets, run via tdnf --installroot, have DNS. The tools chroot's own
	// resolv.conf is handled separately by initToolsChroot.
	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	for _, configWithBase := range rc.ConfigChain {
		snapshotTime := configWithBase.Config.OS.Packages.SnapshotTime
		if rc.Options.PackageSnapshotTime != "" {
			snapshotTime = rc.Options.PackageSnapshotTime
		}

		err = addRemoveAndUpdatePackages(ctx, rc.BuildDirAbs, configWithBase.BaseConfigPath, configWithBase.Config.OS,
			imageChroot, toolsChroot, rc.Options.RpmsSources, rc.Options.UseBaseImageRpmRepos, distroHandler,
			snapshotTime)
		if err != nil {
			return err
		}
	}

	if err = UpdateHostname(ctx, rc.Hostname, imageChroot); err != nil {
		return err
	}

	if err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, rc.ImageUuidStr); err != nil {
		return err
	}

	if err = handleBootLoader(ctx, rc, imageConnection, partitionsLayout, true, distroHandler); err != nil {
		return err
	}

	// Clear systemd state files that should be unique to each instance
	// For the create subcommand, we disable systemd firstboot by default since Azure Linux
	// has traditionally not used firstboot mechanisms.
	err = installutils.ClearSystemdState(imageChroot, false)
	if err != nil {
		return fmt.Errorf("failed to clear systemd state:\n%w", err)
	}

	for _, configWithBase := range rc.ConfigChain {
		err = runUserScripts(ctx, configWithBase.BaseConfigPath, configWithBase.Config.Scripts.PostCustomization,
			"postCustomization", imageChroot)
		if err != nil {
			return err
		}
	}

	if err = restoreResolvConf(ctx, resolvConf, imageChroot); err != nil {
		return err
	}

	if err = checkForInstalledKernel(ctx, imageChroot); err != nil {
		return err
	}

	for _, configWithBase := range rc.ConfigChain {
		err = runUserScripts(ctx, configWithBase.BaseConfigPath, configWithBase.Config.Scripts.FinalizeCustomization,
			"finalizeCustomization", imageChroot)
		if err != nil {
			return err
		}
	}

	return nil
}
