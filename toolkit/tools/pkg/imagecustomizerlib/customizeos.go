// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
)

func DoOsCustomizations(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	imageConnection *ImageConnection, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string,
	diskDevPath string,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format("2006-01-02T15:04:05Z")

	resolvConf, err := overrideResolvConf(imageChroot)
	if err != nil {
		return err
	}

	imageConnection1 := NewImageConnection()

	err = imageConnection1.ConnectLoopback1(*imageConnection)
	if err != nil {
		imageConnection1.Close()
		return fmt.Errorf("failed to connect loopback: %w", err)

	}

	imageChrootDir1 := filepath.Join(imageChroot.RootDir(), "installroot")
	fmt.Println("``````````````````````imageChrootDir1: ", imageChrootDir1)

	err = imageConnection1.ConnectChroot1(imageChrootDir1, true, nil, nil, true)
	if err != nil {
		return fmt.Errorf("failed to connect chroot: %w", err)
	}
	defer imageConnection1.Close()

	imageChroot1 := imageConnection1.Chroot()

	// debugutils.WaitForUser("Install Packages")

	err = addRemoveAndUpdatePackages(buildDir, baseConfigPath, config.OS, imageChroot, rpmsSources,
		useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	// debugutils.WaitForUser("Installed Packages")

	err = UpdateHostname(config.OS.Hostname, imageChroot)
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

	err = AddOrUpdateUsers(config.OS.Users, baseConfigPath, imageChroot)
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

	// debugutils.WaitForUser("DoOsCustomizations: Hard reset bootloader config")

	err = handleBootLoader(baseConfigPath, config, imageConnection1)
	if err != nil {
		return err
	}

	// debugutils.WaitForUser("Configured bootloader")

	selinuxMode, err := handleSELinux(config.OS.SELinux.Mode, config.OS.BootLoader.ResetType,
		imageChroot1)
	if err != nil {
		return err
	}

	overlayUpdated, err := enableOverlays(config.OS.Overlays, selinuxMode, imageChroot1)
	if err != nil {
		return err
	}

	verityUpdated, err := enableVerityPartition(config.Storage.Verity, imageChroot1)
	if err != nil {
		return err
	}

	if partitionsCustomized || overlayUpdated || verityUpdated {
		err = regenerateInitrd(imageChroot1)
		if err != nil {
			return err
		}
	}

	err = runUserScripts(baseConfigPath, config.Scripts.PostCustomization, "postCustomization", imageChroot1)
	if err != nil {
		return err
	}

	err = prepareUki(buildDir, config.OS.Uki, imageChroot1)
	if err != nil {
		return err
	}

	err = restoreResolvConf(resolvConf, imageChroot1)
	if err != nil {
		return err
	}

	err = selinuxSetFiles(selinuxMode, imageChroot1)
	if err != nil {
		return err
	}

	err = runUserScripts(baseConfigPath, config.Scripts.FinalizeCustomization, "finalizeCustomization", imageChroot1)
	if err != nil {
		return err
	}

	err = checkForInstalledKernel(imageChroot1)
	if err != nil {
		return err
	}

	return nil
}
