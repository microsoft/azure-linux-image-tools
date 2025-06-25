// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
)

const (
	buildTimeFormat = "2006-01-02T15:04:05Z"
)

func doOsCustomizations(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	imageConnection *ImageConnection, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string, partUuidToFstabEntry map[string]diskutils.FstabEntry, packageSnapshotTime string,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format(buildTimeFormat)

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	err = addRemoveAndUpdatePackages(buildDir, baseConfigPath, config.OS, imageChroot, nil, rpmsSources,
		useBaseImageRpmRepos, packageSnapshotTime)
	if err != nil {
		return err
	}

	err = UpdateHostname(config.OS.Hostname, imageChroot)
	if err != nil {
		return err
	}

	err = AddOrUpdateUsers(config.OS.Users, baseConfigPath, imageChroot)
	if err != nil {
		return err
	}

	err = copyAdditionalDirs(baseConfigPath, config.OS.AdditionalDirs, imageChroot)
	if err != nil {
		return err
	}

	err = copyAdditionalFiles(baseConfigPath, config.OS.AdditionalFiles, imageChroot)
	if err != nil {
		return err
	}

	err = EnableOrDisableServices(config.OS.Services, imageChroot)
	if err != nil {
		return err
	}

	err = LoadOrDisableModules(config.OS.Modules, imageChroot.RootDir())
	if err != nil {
		return err
	}

	err = addCustomizerRelease(imageChroot.RootDir(), ToolVersion, buildTime, imageUuid)
	if err != nil {
		return err
	}

	if config.OS.ImageHistory != imagecustomizerapi.ImageHistoryNone {
		err = addImageHistory(imageChroot.RootDir(), imageUuid, baseConfigPath, ToolVersion, buildTime, config)
		if err != nil {
			return err
		}
	}

	err = handleBootLoader(baseConfigPath, config, imageConnection, partUuidToFstabEntry, false)
	if err != nil {
		return err
	}

	selinuxMode, err := handleSELinux(config.OS.SELinux.Mode, config.OS.BootLoader.ResetType,
		imageChroot)
	if err != nil {
		return err
	}

	overlayUpdated, err := enableOverlays(config.OS.Overlays, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	verityUpdated, err := enableVerityPartition(config.Storage.Verity, imageChroot)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		err = regenerateInitrd(imageChroot)
		if err != nil {
			return err
		}
	}

	err = runUserScripts(baseConfigPath, config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = prepareUki(buildDir, config.OS.Uki, imageChroot)
	if err != nil {
		return err
	}

	err = restoreResolvConf(resolvConf, imageChroot)
	if err != nil {
		return err
	}

	err = selinuxSetFiles(selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	err = runUserScripts(baseConfigPath, config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot)
	if err != nil {
		return err
	}

	err = checkForInstalledKernel(imageChroot)
	if err != nil {
		return err
	}

	return nil
}
