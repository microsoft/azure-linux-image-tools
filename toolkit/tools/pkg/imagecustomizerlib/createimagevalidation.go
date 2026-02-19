// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"archive/tar"
	"compress/gzip"
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
	rpmsSources []string, toolsTar string, outputImageFile, outputImageFormat string, packageSnapshotTime string,
	buildDir string,
) (*ResolvedConfig, error) {
	if !slices.Contains(config.PreviewFeatures, imagecustomizerapi.PreviewFeatureCreate) {
		return nil, fmt.Errorf(
			"the 'create' feature is currently in preview; please add 'create' to 'previewFeatures' to enable it")
	}

	err := validateCreateImageSupportedFields(config)
	if err != nil {
		return nil, fmt.Errorf("invalid config file (%s):\n%w", baseConfigPath, err)
	}

	// Validate mandatory fields for creating a seed image
	err = validateCreateImageMandatoryFields(baseConfigPath, config, rpmsSources, toolsTar)
	if err != nil {
		return nil, err
	}

	// TODO: Validate for distro and release
	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, false,
		imagecustomizerapi.ValidateResourceTypes{
			imagecustomizerapi.ValidateResourceTypeAll,
		},
		ImageCustomizerOptions{
			RpmsSources:         rpmsSources,
			OutputImageFile:     outputImageFile,
			OutputImageFormat:   imagecustomizerapi.ImageFormatType(outputImageFormat),
			PackageSnapshotTime: imagecustomizerapi.PackageSnapshotTime(packageSnapshotTime),
			BuildDir:            buildDir,
		})
	if err != nil {
		return nil, err
	}

	if len(config.OS.Packages.Install) == 0 {
		return nil, fmt.Errorf(
			"no packages to install specified, please specify at least one package to install for a new image")
	}

	return rc, nil
}

func validateCreateImageMandatoryFields(baseConfigPath string, config *imagecustomizerapi.Config,
	rpmsSources []string, toolsTar string,
) error {
	// check if storage disks is not empty for creating a seed image
	if len(config.Storage.Disks) == 0 {
		return fmt.Errorf("storage.disks field is required in the config file (%s)", baseConfigPath)
	}

	// rpmSources must not be empty for creating a seed image
	if len(rpmsSources) == 0 {
		return fmt.Errorf("rpm sources must be specified for creating a seed image")
	}

	err := validateToolsTarFile(toolsTar)
	if err != nil {
		return err
	}

	return nil
}

func validateToolsTarFile(toolsTar string) error {
	// Check if the tools tar file exists
	if toolsTar == "" {
		return fmt.Errorf("tools tar file is required for image creation")
	}
	if _, err := os.Stat(toolsTar); os.IsNotExist(err) {
		return fmt.Errorf("tools tar file (%s) does not exist", toolsTar)
	}
	// Check if the tools tar file is a valid tar file
	isValid, err := isValidTarGz(toolsTar)
	if err != nil {
		return fmt.Errorf("failed to validate tools tar file (%s): %w", toolsTar, err)
	}
	if !isValid {
		return fmt.Errorf("tools tar file (%s) is not a valid tar.gz file", toolsTar)
	}

	return nil
}

func isValidTarGz(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Check gzip header
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return false, fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gzReader.Close()

	// Check tar structure
	tarReader := tar.NewReader(gzReader)
	_, err = tarReader.Next()
	if err != nil {
		return false, fmt.Errorf("not a valid tar archive: %w", err)
	}

	return true, nil
}
