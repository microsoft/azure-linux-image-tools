// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
)

const (
	buildTimeFormat = "2006-01-02T15:04:05Z"
)

func doOsCustomizations(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	imageConnection *imageconnection.ImageConnection, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string, partUuidToFstabEntry map[string]diskutils.FstabEntry, packageSnapshotTime string,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	err = addRemoveAndUpdatePackages(ctx, buildDir, baseConfigPath, config.OS, imageChroot, nil, rpmsSources,
		useBaseImageRpmRepos, packageSnapshotTime)
	if err != nil {
		return err
	}

	err = UpdateHostname(ctx, config.OS.Hostname, imageChroot)
	if err != nil {
		return err
	}

	err = AddOrUpdateGroups(ctx, config.OS.Groups, imageChroot)
	if err != nil {
		return err
	}

	err = AddOrUpdateUsers(ctx, config.OS.Users, baseConfigPath, imageChroot)
	if err != nil {
		return err
	}

	err = copyAdditionalDirs(ctx, baseConfigPath, config.OS.AdditionalDirs, imageChroot)
	if err != nil {
		return err
	}

	err = copyAdditionalFiles(ctx, baseConfigPath, config.OS.AdditionalFiles, imageChroot)
	if err != nil {
		return err
	}

	err = EnableOrDisableServices(ctx, config.OS.Services, imageChroot)
	if err != nil {
		return err
	}

	err = LoadOrDisableModules(ctx, config.OS.Modules, imageChroot.RootDir())
	if err != nil {
		return err
	}

	err = addCustomizerRelease(ctx, imageChroot.RootDir(), ToolVersion, buildTime, imageUuid)
	if err != nil {
		return err
	}

	if config.OS.ImageHistory != imagecustomizerapi.ImageHistoryNone {
		err = addImageHistory(ctx, imageChroot, imageUuid, baseConfigPath, ToolVersion, buildTime, config)
		if err != nil {
			return err
		}
	}

	err = handleBootLoader(ctx, baseConfigPath, config, imageConnection, partUuidToFstabEntry, false)
	if err != nil {
		return err
	}

	selinuxMode, err := handleSELinux(ctx, config.OS.SELinux.Mode, config.OS.BootLoader.ResetType,
		imageChroot)
	if err != nil {
		return err
	}

	overlayUpdated, err := enableOverlays(ctx, config.OS.Overlays, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	verityUpdated, err := enableVerityPartition(ctx, config.Storage.Verity, imageChroot)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		err = regenerateInitrd(ctx, imageChroot)
		if err != nil {
			return err
		}
	}

	err = runUserScripts(ctx, baseConfigPath, config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = prepareUki(ctx, buildDir, config.OS.Uki, imageChroot)
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

	err = runUserScripts(ctx, baseConfigPath, config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = checkForInstalledKernel(ctx, imageChroot)
	if err != nil {
		return err
	}

	return nil
}
