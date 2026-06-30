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

func validateCreateImageSupportedFields(rc *ResolvedConfig) error {
	// Verify that the config file does not contain any fields that are not supported by the create subcommand.
	if rc.InputImage != (imagecustomizerapi.InputImage{}) {
		return fmt.Errorf("input field is not supported by the create subcommand")
	}

	if rc.Storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault {
		return fmt.Errorf("reset partitions uuids field is not supported by the create subcommand")
	}

	if rc.Storage.Verity != nil {
		return fmt.Errorf("storage verity field is not supported by the create subcommand")
	}

	if rc.Uki != nil {
		return fmt.Errorf("uki field is not supported by the create subcommand")
	}

	for _, config := range rc.ConfigChain {
		if config.Config.OS != nil {
			if err := validateCreateImageSupportedOsFields(config.Config.OS); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateCreateImageSupportedOsFields(osConfig *imagecustomizerapi.OS) error {
	return nil
}

func validateCreateImageConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config,
	options ImageCreateOptions,
) (*ResolvedConfig, error) {
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

	err = validateCreateImageSupportedFields(rc)
	if err != nil {
		return nil, fmt.Errorf("invalid config file (%s):\n%w", baseConfigPath, err)
	}

	// Validate mandatory fields for creating a seed image
	err = validateCreateImageMandatoryFields(baseConfigPath, rc)
	if err != nil {
		return nil, err
	}

	if len(config.OS.Packages.Install) == 0 {
		return nil, fmt.Errorf(
			"no packages to install specified, please specify at least one package to install for a new image")
	}

	return rc, nil
}

func validateCreateImageMandatoryFields(baseConfigPath string, rc *ResolvedConfig) error {
	// check if storage disks is not empty for creating a seed image
	if len(rc.Storage.Disks) == 0 {
		return fmt.Errorf("storage.disks field is required in the config file (%s)", baseConfigPath)
	}

	// rpmSources must not be empty for creating a seed image
	if len(rc.Options.RpmsSources) == 0 {
		return fmt.Errorf("rpm sources must be specified for creating a seed image")
	}

	if rc.Options.ToolsDir == "" {
		return fmt.Errorf("tools directory is required for image creation")
	}
	err := validateToolsDir(rc.Options.ToolsDir)
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
