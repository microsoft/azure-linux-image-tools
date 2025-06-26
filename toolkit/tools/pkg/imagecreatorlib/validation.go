package imagecreatorlib

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
)

func validateSupportedFields(c *imagecustomizerapi.Config) error {
	// Verify that the config file does not contain any fields that are not supported by the image creator.
	if c.Input != (imagecustomizerapi.Input{}) {
		return fmt.Errorf("input field is not supported by the image creator")
	}
	if c.Iso != nil {
		return fmt.Errorf("iso field is not supported by the image creator")
	}
	if c.Pxe != nil {
		return fmt.Errorf("pxe field is not supported by the image creator")
	}

	if len(c.PreviewFeatures) > 0 {
		return fmt.Errorf("preview features field is not supported by the image creator")
	}

	if c.Storage.ResetPartitionsUuidsType != imagecustomizerapi.ResetPartitionsUuidsTypeDefault {
		return fmt.Errorf("reset partitions uuids field is not supported by the image creator")
	}

	if c.Storage.Verity != nil {
		return fmt.Errorf("storage verity field is not supported by the image creator")
	}

	if c.OS != nil && len(c.OS.AdditionalFiles) > 0 {
		return fmt.Errorf("os.additionalFiles field is not supported by the image creator")
	}

	if c.OS != nil && len(c.OS.AdditionalDirs) > 0 {
		return fmt.Errorf("os.additionalDirectories field is not supported by the image creator")
	}

	if c.OS != nil && c.OS.Uki != nil {
		return fmt.Errorf("uki field is not supported by the image creator")
	}

	if c.OS != nil && c.OS.SELinux != (imagecustomizerapi.SELinux{}) {
		return fmt.Errorf("selinux field is not supported by the image creator")
	}

	if c.OS != nil && len(c.OS.Modules) > 0 {
		return fmt.Errorf("os.modules field is not supported by the image creator")
	}
	/*
		if c.OS != nil && len(c.OS.KernelCommandLine.ExtraCommandLine) > 0 {
			return fmt.Errorf("os.kernelCommandLine field is not supported by the image creator")
		}
	*/

	if c.OS != nil && c.OS.Overlays != nil {
		return fmt.Errorf("os.overlay field is not supported by the image creator")
	}
	return nil
}

func validateConfig(baseConfigPath string, config *imagecustomizerapi.Config, rpmsSources []string,
	outputImageFile, outputImageFormat string, packageSnapshotTime string,
) error {
	err := validateSupportedFields(config)
	if err != nil {
		return fmt.Errorf("invalid config file %s:\n%w", baseConfigPath, err)
	}

	// check if storage disks is not empty for creating a seed image
	if len(config.Storage.Disks) == 0 {
		return fmt.Errorf("storage.disks field is required in the config file %s", baseConfigPath)
	}

	// rpmSources must not be empty for creating a seed image
	if len(rpmsSources) == 0 {
		return fmt.Errorf("rpm sources must be specified for creating a seed image")
	}

	// TODO: Validate for distro and release
	err = imagecustomizerlib.ValidateConfig(baseConfigPath, config,
		"", rpmsSources, outputImageFile, outputImageFormat, false, packageSnapshotTime, true)
	if err != nil {
		return err
	}

	return nil
}
