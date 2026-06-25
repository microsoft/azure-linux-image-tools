// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func validateCreateImageSupportedFields(c *imagecustomizerapi.Config) error {
	// Verify that the config file does not contain any fields that are not supported by the create subcommand.
	if c.Input != (imagecustomizerapi.Input{}) {
		return fmt.Errorf("input field is not supported by the create subcommand")
	}
	if c.Iso != nil {
		return fmt.Errorf("iso field is not supported by the create subcommand")
	}
	if c.Pxe != nil {
		return fmt.Errorf("pxe field is not supported by the create subcommand")
	}

	if c.Storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault {
		return fmt.Errorf("reset partitions uuids field is not supported by the create subcommand")
	}

	if c.Storage.Verity != nil {
		return fmt.Errorf("storage verity field is not supported by the create subcommand")
	}

	if c.OS != nil {
		if err := validateCreateImageSupportedOsFields(c.OS); err != nil {
			return err
		}
	}
	return nil
}

func validateCreateImageSupportedOsFields(osConfig *imagecustomizerapi.OS) error {
	if len(osConfig.AdditionalFiles) > 0 {
		return fmt.Errorf("os.additionalFiles field is not supported by the create subcommand")
	}

	if len(osConfig.AdditionalDirs) > 0 {
		return fmt.Errorf("os.additionalDirectories field is not supported by the create subcommand")
	}

	if osConfig.Uki != nil {
		return fmt.Errorf("uki field is not supported by the create subcommand")
	}

	if osConfig.SELinux != (imagecustomizerapi.SELinux{}) {
		return fmt.Errorf("selinux field is not supported by the create subcommand")
	}

	if len(osConfig.Modules) > 0 {
		return fmt.Errorf("os.modules field is not supported by the create subcommand")
	}

	if osConfig.Overlays != nil {
		return fmt.Errorf("os.overlay field is not supported by the create subcommand")
	}
	return nil
}

func validateCreateImageConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config,
	options ImageCreateOptions,
) (*ResolvedConfig, error) {
	err := validateCreateImageSupportedFields(config)
	if err != nil {
		return nil, fmt.Errorf("invalid config file (%s):\n%w", baseConfigPath, err)
	}

	// Validate mandatory fields for creating a seed image
	err = validateCreateImageMandatoryFields(baseConfigPath, config, options.RpmsSources, options.ToolsDir)
	if err != nil {
		return nil, err
	}

	// TODO: Validate for distro and release
	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, false,
		imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeAll,
		},
		ImageCustomizerOptions{
			RpmsSources:         options.RpmsSources,
			OutputImageFile:     options.OutputImageFile,
			OutputImageFormat:   options.OutputImageFormat,
			PackageSnapshotTime: options.PackageSnapshotTime,
			BuildDir:            options.BuildDir,
			ToolsDir:            options.ToolsDir,
			PreviewFeatures:     options.PreviewFeatures,
		})
	if err != nil {
		return nil, err
	}

	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureCreate) {
		return nil, fmt.Errorf(
			"the 'create' feature is currently in preview; please add 'create' to 'previewFeatures' to enable it")
	}

	if len(config.OS.Packages.Install) == 0 {
		return nil, fmt.Errorf(
			"no packages to install specified, please specify at least one package to install for a new image")
	}

	return rc, nil
}

func validateCreateImageMandatoryFields(baseConfigPath string, config *imagecustomizerapi.Config,
	rpmsSources []string, toolsDir string,
) error {
	// check if storage disks is not empty for creating a seed image
	if len(config.Storage.Disks) == 0 {
		return fmt.Errorf("storage.disks field is required in the config file (%s)", baseConfigPath)
	}

	// rpmSources must not be empty for creating a seed image
	if len(rpmsSources) == 0 {
		return fmt.Errorf("rpm sources must be specified for creating a seed image")
	}

	if toolsDir == "" {
		return fmt.Errorf("tools directory is required for image creation")
	}
	err := validateToolsDir(toolsDir)
	if err != nil {
		return err
	}

	return nil
}

func validateToolsDir(toolsDir string) error {
	info, err := os.Stat(toolsDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("tools directory (%s) does not exist", toolsDir)
	}
	if err != nil {
		return fmt.Errorf("failed to stat tools directory (%s):\n%w", toolsDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("tools path (%s) is not a directory", toolsDir)
	}

	return nil
}
