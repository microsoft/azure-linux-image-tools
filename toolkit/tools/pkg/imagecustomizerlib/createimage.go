// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
)

const (
	setupRoot = "/setuproot"
)

func CreateImageWithConfigFile(ctx context.Context, buildDir string, configFile string, rpmsSources []string,
	toolsTar string, outputImageFile string, outputImageFormat string, distro string, distroVersion string,
	packageSnapshotTime string,
) error {
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config file %s:\n%w", configFile, err)
	}

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of config file directory:\n%w", err)
	}

	err = createNewImage(
		ctx, buildDir, absBaseConfigPath, config, rpmsSources, outputImageFile,
		outputImageFormat, toolsTar, distro, distroVersion, packageSnapshotTime)
	if err != nil {
		return err
	}

	return nil
}

func createNewImage(ctx context.Context, buildDir string, baseConfigPath string, config imagecustomizerapi.Config,
	rpmsSources []string, outputImageFile string, outputImageFormat string, toolsTar string, distro string,
	distroVersion string, packageSnapshotTime string,
) error {
	rc, err := validateCreateImageConfig(
		ctx, baseConfigPath, &config, rpmsSources, toolsTar, outputImageFile,
		outputImageFormat, packageSnapshotTime, buildDir)
	if err != nil {
		return err
	}

	err = CheckEnvironmentVars()
	if err != nil {
		return err
	}

	LogVersionsOfToolDeps()

	outputImageDir := filepath.Dir(rc.OutputImageFile)
	err = os.MkdirAll(outputImageDir, os.ModePerm)
	if err != nil {
		return err
	}

	disks := rc.Config.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	logger.Log.Infof("Creating new image with parameters: %+v\n", rc)

	// Create distro config from distro name and version
	distroHandler := NewDistroHandler(distro, distroVersion)

	partIdToPartUuid, err := CreateNewImage(
		distroHandler.GetTargetOs(), rc.RawImageFile,
		diskConfig, rc.Config.Storage.FileSystems,
		rc.BuildDirAbs, setupRoot, installOSFunc)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Part id to part uuid map %v\n", partIdToPartUuid)
	logger.Log.Infof("Image UUID: %s", rc.ImageUuidStr)

	partUuidToFstabEntry, osRelease, err := CustomizeImageHelperCreate(ctx, rc, toolsTar,
		distroHandler)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Part uuid to fstab entry: %v\n", partUuidToFstabEntry)
	logger.Log.Debugf("OsRelease: %v\n", osRelease)

	logger.Log.Infof("Writing: %s", rc.OutputImageFile)

	err = ConvertImageFile(rc.RawImageFile, rc.OutputImageFile, rc.OutputImageFormat)
	if err != nil {
		return err
	}
	logger.Log.Infof("Success!")

	return nil
}
