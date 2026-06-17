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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

const (
	setupRoot = "/setuproot"
)

type ImageCreateOptions struct {
	BuildDir            string
	Distro              targetos.Distro
	DistroVersion       string
	ToolsTar            string
	RpmsSources         []string
	OutputImageFile     string
	OutputImageFormat   imagecustomizerapi.ImageFormatType
	PackageSnapshotTime imagecustomizerapi.PackageSnapshotTime

	// Not provided via the command line. Only used in tests.
	PreviewFeatures []imagecustomizerapi.PreviewFeature
}

func CreateImageWithConfigFile(ctx context.Context, configFile string, options ImageCreateOptions) error {
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

	err = CreateImage(ctx, absBaseConfigPath, config, options)
	if err != nil {
		return err
	}

	return nil
}

func CreateImage(ctx context.Context, baseConfigPath string, config imagecustomizerapi.Config,
	options ImageCreateOptions,
) error {
	rc, err := validateCreateImageConfig(ctx, baseConfigPath, &config, options)
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

	disks := rc.Storage.Disks
	diskConfig := disks[0]
	installOSFunc := func(imageChroot *safechroot.Chroot) error {
		return nil
	}

	logger.Log.Infof("Creating new image with parameters: %+v\n", rc)

	// Create distro config from distro name and version
	distroHandler, err := NewDistroHandler(targetos.New(options.Distro, options.DistroVersion))
	if err != nil {
		return err
	}

	// Validate distro specific settings.
	err = distroHandler.ValidateConfig(rc)
	if err != nil {
		return fmt.Errorf("invalid config for image distro:\n%w", err)
	}

	partIdToPartUuid, err := CreateNewImage(
		distroHandler, rc.RawImageFile,
		diskConfig, rc.Storage.FileSystems,
		rc.BuildDirAbs, setupRoot, installOSFunc)
	if err != nil {
		return err
	}

	logger.Log.Debugf("Part id to part uuid map %v\n", partIdToPartUuid)
	logger.Log.Infof("Image UUID: %s", rc.ImageUuidStr)

	partUuidToFstabEntry, osRelease, err := CustomizeImageHelperCreate(ctx, rc, options.ToolsTar,
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
