// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/debugutils"
	"golang.org/x/sys/unix"
)

func DoOsCustomizations(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	imageConnection *ImageConnection, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string,
	diskDevPath string,
) error {
	var err error

	imageChroot := imageConnection.Chroot()

	buildTime := time.Now().Format("2006-01-02T15:04:05Z")

	imageConnection1 := NewImageConnection()

	err = imageConnection1.ConnectLoopback1(*imageConnection)
	if err != nil {
		imageConnection1.Close()
		return fmt.Errorf("failed to connect loopback: %w", err)

	}

	imageChrootDir1 := filepath.Join(buildDir, "installroot")
	fmt.Println("``````````````````````imageChrootDir1: ", imageChrootDir1)

	err = imageConnection1.ConnectChroot1(imageChrootDir1, false, nil, nil, true)
	if err != nil {
		return fmt.Errorf("failed to connect chroot: %w", err)
	}
	defer imageConnection1.Close()

	source := imageChroot.RootDir()
	target := filepath.Join(imageChrootDir1, "imageroot")
	// mkdir -p target
	err = os.MkdirAll(target, 0o755)
	if err != nil {
		fmt.Println("mkdir failed: ", err)
		// Close chroot1 and chroor2
		imageConnection1.Close()
		imageConnection.Close()
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	fmt.Println("source: ", source)
	fmt.Println("target: ", target)
	// Perform bind mount
	err = unix.Mount(source, target, "", unix.MS_BIND, "")
	if err != nil {
		// Close chroot1 and chroor2
		imageConnection1.Close()
		imageConnection.Close()
		log.Fatalf("Bind mount failed: %v", err)
	}

	fmt.Printf("Successfully bind-mounted %s to %s\n", source, target)

	imageChroot1 := imageConnection1.Chroot()

	// debugutils.WaitForUser("Install Packages")

	resolvConf, err := overrideResolvConf(imageChroot1)
	if err != nil {
		return err
	}

	err = addRemoveAndUpdatePackages(buildDir, baseConfigPath, config.OS, imageChroot1, rpmsSources,
		useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	fmt.Printf("Packages installed successfully in %s\n", imageChroot1.RootDir())
	fmt.Printf("unmounting target %s\n", target)
	// Remove the bind mount after package installation
	err = unix.Unmount(target, unix.MNT_DETACH)
	if err != nil {
		log.Fatalf("Unmount failed: %v", err)
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

	debugutils.WaitForUser("DoOsCustomizations: Hard reset bootloader config")

	err = handleBootLoader(baseConfigPath, config, imageConnection)
	if err != nil {
		return err
	}

	debugutils.WaitForUser("Configured bootloader")

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
