// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

func doOsCustomizations(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	imageConnection *ImageConnection, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format("2006-01-02T15:04:05Z")

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Installing packages")

	err = addRemoveAndUpdatePackages(buildDir, baseConfigPath, config.OS, imageChroot, rpmsSources,
		useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Updating hostname")

	err = UpdateHostname(config.OS.Hostname, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Copying additional files and dirs")

	err = copyAdditionalDirs(baseConfigPath, config.OS.AdditionalDirs, imageChroot)
	if err != nil {
		return err
	}

	err = copyAdditionalFiles(baseConfigPath, config.OS.AdditionalFiles, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Updating users")

	err = AddOrUpdateUsers(config.OS.Users, baseConfigPath, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Enabling/Disabling services")

	err = EnableOrDisableServices(config.OS.Services, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Enabling/Disabling modules")

	err = LoadOrDisableModules(config.OS.Modules, imageChroot.RootDir())
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Customizing release")

	err = addCustomizerRelease(imageChroot.RootDir(), ToolVersion, buildTime, imageUuid)
	if err != nil {
		return err
	}

	if config.OS.ImageHistory != imagecustomizerapi.ImageHistoryNone {

		logger.Log.Infof("OS - Adding image history")

		err = addImageHistory(imageChroot.RootDir(), imageUuid, baseConfigPath, ToolVersion, buildTime, config)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("OS - Handle bootloader")

	err = handleBootLoader(baseConfigPath, config, imageConnection)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Handle selinux")

	selinuxMode, err := handleSELinux(config.OS.SELinux.Mode, config.OS.BootLoader.ResetType,
		imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Enable overlays")

	overlayUpdated, err := enableOverlays(config.OS.Overlays, selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Enable verity partition")

	verityUpdated, err := enableVerityPartition(config.Storage.Verity, imageChroot)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		logger.Log.Infof("OS - Regenerate initd")

		err = regenerateInitrd(imageChroot)
		if err != nil {
			return err
		}
	}

	logger.Log.Infof("OS - Run user postCustomization")

	err = runUserScripts(baseConfigPath, config.Scripts.PostCustomization, "postCustomization", imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Prepare uki")

	err = prepareUki(buildDir, config.OS.Uki, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Restore resolve conf")

	err = restoreResolvConf(resolvConf, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Set selinux files")

	err = selinuxSetFiles(selinuxMode, imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Run user finalizeCustomization")

	err = runUserScripts(baseConfigPath, config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot)
	if err != nil {
		return err
	}

	logger.Log.Infof("OS - Check installed kernel")

	err = checkForInstalledKernel(imageChroot)
	if err != nil {
		return err
	}

	return nil
}
