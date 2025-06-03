// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"golang.org/x/sys/unix"
)

func CustomizeImageHelperImager(buildDir string, baseConfigPath string, config *imagecustomizerapi.Config,
	rawImageFile string, rpmsSources []string, useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuidStr string,
	diskDevPath string, packageSnapshotTime string, tarFile string,
) (map[string]diskutils.FstabEntry, string, error) {
	logger.Log.Debugf("Customizing OS with imager")

	imageConnection, partUuidToFstabEntry, _, err := connectToExistingImage(rawImageFile, buildDir, imageRoot, true)
	if err != nil {
		return nil, "", err
	}
	defer imageConnection.Close()

	// Do the actual customizations.
	err = DoOsCustomizationsImager(buildDir, baseConfigPath, config, imageConnection, rpmsSources,
		useBaseImageRpmRepos, partitionsCustomized, imageUuidStr,
		diskDevPath, partUuidToFstabEntry, packageSnapshotTime, tarFile)
	// Out of disk space errors can be difficult to diagnose.
	// So, warn about any partitions with low free space.

	warnOnLowFreeSpace(buildDir, imageConnection)
	if err != nil {
		return nil, "", err
	}

	err = imageConnection.CleanClose()
	if err != nil {
		return nil, "", err
	}

	return partUuidToFstabEntry, "", nil
}

func DoOsCustomizationsImager(
	buildDir string, baseConfigPath string,
	config *imagecustomizerapi.Config,
	imageConnection *ImageConnection,
	rpmsSources []string,
	useBaseImageRpmRepos bool, partitionsCustomized bool,
	imageUuid string, diskDevPath string,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
	packageSnapshotTime string, tarfile string,
) error {
	imageChroot := imageConnection.Chroot()
	buildTime := time.Now().Format("2006-01-02T15:04:05Z")
	toolsChrootDir := filepath.Join(buildDir, toolsRoot)
	source := imageChroot.RootDir()
	target := filepath.Join(toolsChrootDir, imageRoot)

	toolsChroot, err := safechroot.CreateToolsChroot(toolsChrootDir, false, nil, nil, true, tarfile)
	if err != nil {
		return fmt.Errorf("failed to create tools chroot: %w", err)
	}
	defer toolsChroot.Close(false)

	if err = os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	if err = unix.Mount(source, target, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mount failed: %w", err)
	}
	defer func() {
		_ = unix.Unmount(target, unix.MNT_DETACH)
	}()

	resolvConf, err := overrideResolvConf(toolsChroot)
	if err != nil {
		return err
	}

	if err := addRemoveAndUpdatePackages(
		buildDir, baseConfigPath, config.OS, toolsChroot, rpmsSources,
		useBaseImageRpmRepos, packageSnapshotTime); err != nil {
		return err
	}

	if err := unix.Unmount(target, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("bind unmount failed: %w", err)
	}

	if err := toolsChroot.Close(false); err != nil {
		return fmt.Errorf("failed to close tools chroot: %w", err)
	}

	if err := UpdateHostname(config.OS.Hostname, imageChroot); err != nil {
		return err
	}

	if err := addCustomizerRelease(imageChroot.RootDir(), ToolVersion, buildTime, imageUuid); err != nil {
		return err
	}

	if err := handleBootLoader(baseConfigPath, config, imageConnection, partUuidToFstabEntry, true); err != nil {
		return err
	}

	if err := prepareUki(buildDir, config.OS.Uki, imageChroot); err != nil {
		return err
	}

	if err := restoreResolvConf(resolvConf, imageChroot); err != nil {
		return err
	}

	if err := checkForInstalledKernel(imageChroot); err != nil {
		return err
	}

	return nil
}
