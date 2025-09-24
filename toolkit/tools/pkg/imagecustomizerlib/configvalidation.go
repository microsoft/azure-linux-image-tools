// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"go.opentelemetry.io/otel"
)

var (
	// Validation errors
	ErrInputImageFileRequired         = NewImageCustomizerError("Validation:InputImageFileRequired", "input image file must be specified")
	ErrInvalidInputImageFileArg       = NewImageCustomizerError("Validation:InvalidInputImageFileArg", "invalid command-line option '--image-file'")
	ErrInputImageFileNotFile          = NewImageCustomizerError("Validation:InputImageFileNotFile", "input image file is not a file")
	ErrInvalidInputImageFileConfig    = NewImageCustomizerError("Validation:InvalidInputImageFileConfig", "invalid config file property 'input.image.path'")
	ErrInvalidAdditionalFilesSource   = NewImageCustomizerError("Validation:InvalidAdditionalFilesSource", "invalid additionalFiles source file")
	ErrAdditionalFilesSourceNotFile   = NewImageCustomizerError("Validation:AdditionalFilesSourceNotFile", "additionalFiles source file is not a file")
	ErrInvalidPostCustomizationScript = NewImageCustomizerError("Validation:InvalidPostCustomizationScript", "invalid postCustomization script")
	ErrInvalidFinalizeScript          = NewImageCustomizerError("Validation:InvalidFinalizeScript", "invalid finalizeCustomization script")
	ErrScriptNotUnderConfigDir        = NewImageCustomizerError("Validation:ScriptNotUnderConfigDir", "script file is not under config directory")
	ErrScriptFileNotReadable          = NewImageCustomizerError("Validation:ScriptFileNotReadable", "couldn't read script file")
	ErrNoRpmSourcesSpecified          = NewImageCustomizerError("Validation:NoRpmSourcesSpecified", "have packages to install or update but no RPM sources were specified")
	ErrOutputImageFileRequired        = NewImageCustomizerError("Validation:OutputImageFileRequired", "output image file must be specified")
	ErrInvalidOutputImageFileArg      = NewImageCustomizerError("Validation:InvalidOutputImageFileArg", "invalid command-line option '--output-image-file'")
	ErrOutputImageFileIsDirectory     = NewImageCustomizerError("Validation:OutputImageFileIsDirectory", "output image file is a directory")
	ErrInvalidOutputImageFileConfig   = NewImageCustomizerError("Validation:InvalidOutputImageFileConfig", "invalid config file property 'output.image.path'")
	ErrOutputImageFormatRequired      = NewImageCustomizerError("Validation:OutputImageFormatRequired", "output image format must be specified")
	ErrInvalidUser                    = NewImageCustomizerError("Validation:InvalidUser", "invalid user")
	ErrInvalidSSHPublicKeyFile        = NewImageCustomizerError("Validation:InvalidSSHPublicKeyFile", "failed to find SSH public key file")
	ErrSSHPublicKeyNotFile            = NewImageCustomizerError("Validation:SSHPublicKeyNotFile", "SSH public key path is not a file")
	ErrInvalidPackageSnapshotTime     = NewImageCustomizerError("Validation:InvalidPackageSnapshotTime", "invalid command-line option '--package-snapshot-time'")
)

func ValidateConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config, inputImageFile string, rpmsSources []string,
	outputImageFile, outputImageFormat string, useBaseImageRpmRepos bool, packageSnapshotTime string, newImage bool,
) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "validate_config")
	defer span.End()

	err := config.IsValid()
	if err != nil {
		return err
	}

	if !newImage {
		err = validateInput(baseConfigPath, config.Input, inputImageFile)
		if err != nil {
			return err
		}
	}

	err = validateIsoConfig(baseConfigPath, config.Iso)
	if err != nil {
		return err
	}

	err = validateOsConfig(baseConfigPath, config.OS, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	err = validateScripts(baseConfigPath, &config.Scripts)
	if err != nil {
		return err
	}

	err = validateOutput(baseConfigPath, config.Output, outputImageFile, outputImageFormat)
	if err != nil {
		return err
	}

	if err := validateSnapshotTimeInput(packageSnapshotTime, config.PreviewFeatures); err != nil {
		return err
	}

	return nil
}

func validateInput(baseConfigPath string, input imagecustomizerapi.Input, inputImageFile string) error {
	if inputImageFile == "" && input.Image.Path == "" {
		return ErrInputImageFileRequired
	}

	if inputImageFile != "" {
		if yes, err := file.IsFile(inputImageFile); err != nil {
			return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidInputImageFileArg, inputImageFile, err)
		} else if !yes {
			return fmt.Errorf("%w (file='%s')", ErrInputImageFileNotFile, inputImageFile)
		}
	} else {
		inputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, input.Image.Path)
		if yes, err := file.IsFile(inputImageAbsPath); err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidInputImageFileConfig, input.Image.Path, err)
		} else if !yes {
			return fmt.Errorf("%w (path='%s')", ErrInputImageFileNotFile, input.Image.Path)
		}
	}

	return nil
}

func validateAdditionalFiles(baseConfigPath string, additionalFiles imagecustomizerapi.AdditionalFileList) error {
	errs := []error(nil)
	for _, additionalFile := range additionalFiles {
		switch {
		case additionalFile.Source != "":
			sourceFileFullPath := file.GetAbsPathWithBase(baseConfigPath, additionalFile.Source)
			isFile, err := file.IsFile(sourceFileFullPath)
			if err != nil {
				errs = append(errs, fmt.Errorf("%w (source='%s'):\n%w", ErrInvalidAdditionalFilesSource, additionalFile.Source, err))
			}

			if !isFile {
				errs = append(errs, fmt.Errorf("%w (source='%s')", ErrAdditionalFilesSourceNotFile,
					additionalFile.Source))
			}
		}
	}

	return errors.Join(errs...)
}

func validateIsoConfig(baseConfigPath string, config *imagecustomizerapi.Iso) error {
	if config == nil {
		return nil
	}

	err := validateAdditionalFiles(baseConfigPath, config.AdditionalFiles)
	if err != nil {
		return err
	}

	return nil
}

func validateOsConfig(baseConfigPath string, config *imagecustomizerapi.OS,
	rpmsSources []string, useBaseImageRpmRepos bool,
) error {
	if config == nil {
		return nil
	}

	var err error

	err = validatePackageLists(baseConfigPath, config, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return err
	}

	err = validateAdditionalFiles(baseConfigPath, config.AdditionalFiles)
	if err != nil {
		return err
	}

	err = validateUsers(baseConfigPath, config.Users)
	if err != nil {
		return err
	}

	return nil
}

func validateScripts(baseConfigPath string, scripts *imagecustomizerapi.Scripts) error {
	if scripts == nil {
		return nil
	}

	for i, script := range scripts.PostCustomization {
		err := validateScript(baseConfigPath, &script)
		if err != nil {
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidPostCustomizationScript, i, err)
		}
	}

	for i, script := range scripts.FinalizeCustomization {
		err := validateScript(baseConfigPath, &script)
		if err != nil {
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidFinalizeScript, i, err)
		}
	}

	return nil
}

func validateScript(baseConfigPath string, script *imagecustomizerapi.Script) error {
	if script.Path != "" {
		// Ensure that install scripts sit under the config file's parent directory.
		// This allows the install script to be run in the chroot environment by bind mounting the config directory.
		if !filepath.IsLocal(script.Path) {
			return fmt.Errorf("%w (script='%s', config='%s')", ErrScriptNotUnderConfigDir, script.Path, baseConfigPath)
		}

		fullPath := filepath.Join(baseConfigPath, script.Path)

		// Verify that the file exists.
		_, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("%w (script='%s'):\n%w", ErrScriptFileNotReadable, script.Path, err)
		}
	}

	return nil
}

func validatePackageLists(baseConfigPath string, config *imagecustomizerapi.OS, rpmsSources []string,
	useBaseImageRpmRepos bool,
) error {
	if config == nil {
		return nil
	}

	allPackagesRemove, err := collectPackagesList(baseConfigPath, config.Packages.RemoveLists, config.Packages.Remove)
	if err != nil {
		return err
	}

	allPackagesInstall, err := collectPackagesList(baseConfigPath, config.Packages.InstallLists, config.Packages.Install)
	if err != nil {
		return err
	}

	allPackagesUpdate, err := collectPackagesList(baseConfigPath, config.Packages.UpdateLists, config.Packages.Update)
	if err != nil {
		return err
	}

	hasRpmSources := len(rpmsSources) > 0 || useBaseImageRpmRepos

	if !hasRpmSources {
		needRpmsSources := len(allPackagesInstall) > 0 || len(allPackagesUpdate) > 0 ||
			config.Packages.UpdateExistingPackages

		if needRpmsSources {
			return ErrNoRpmSourcesSpecified
		}
	}

	config.Packages.Remove = allPackagesRemove
	config.Packages.Install = allPackagesInstall
	config.Packages.Update = allPackagesUpdate

	config.Packages.RemoveLists = nil
	config.Packages.InstallLists = nil
	config.Packages.UpdateLists = nil

	return nil
}

func validateOutput(baseConfigPath string, output imagecustomizerapi.Output, outputImageFile, outputImageFormat string) error {
	if outputImageFile == "" && output.Image.Path == "" {
		return ErrOutputImageFileRequired
	}

	// Pxe output format allows the output to be a path.
	if output.Image.Format != imagecustomizerapi.ImageFormatTypePxeDir && outputImageFormat != string(imagecustomizerapi.ImageFormatTypePxeDir) {
		if outputImageFile != "" {
			if isDir, err := file.DirExists(outputImageFile); err != nil {
				return fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidOutputImageFileArg, outputImageFile, err)
			} else if isDir {
				return fmt.Errorf("%w (file='%s')", ErrOutputImageFileIsDirectory, outputImageFile)
			}
		} else {
			outputImageAbsPath := file.GetAbsPathWithBase(baseConfigPath, output.Image.Path)
			if isDir, err := file.DirExists(outputImageAbsPath); err != nil {
				return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidOutputImageFileConfig, output.Image.Path, err)
			} else if isDir {
				return fmt.Errorf("%w (path='%s')", ErrOutputImageFileIsDirectory, output.Image.Path)
			}
		}
	}

	if outputImageFormat == "" && output.Image.Format == imagecustomizerapi.ImageFormatTypeNone {
		return ErrOutputImageFormatRequired
	}

	return nil
}

func validateUsers(baseConfigPath string, users []imagecustomizerapi.User) error {
	for _, user := range users {
		err := validateUser(baseConfigPath, user)
		if err != nil {
			return fmt.Errorf("%w (user='%s'):\n%w", ErrInvalidUser, user.Name, err)
		}
	}

	return nil
}

func validateUser(baseConfigPath string, user imagecustomizerapi.User) error {
	for _, path := range user.SSHPublicKeyPaths {
		absPath := file.GetAbsPathWithBase(baseConfigPath, path)
		isFile, err := file.IsFile(absPath)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidSSHPublicKeyFile, path, err)
		}
		if !isFile {
			return fmt.Errorf("%w (path='%s')", ErrSSHPublicKeyNotFile, path)
		}
	}

	return nil
}

func validateSnapshotTimeInput(snapshotTime string, previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if snapshotTime != "" && !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeaturePackageSnapshotTime) {
		return ErrPackageSnapshotPreviewRequired
	}

	if err := imagecustomizerapi.PackageSnapshotTime(snapshotTime).IsValid(); err != nil {
		return fmt.Errorf("%w (time='%s'):\n%w", ErrInvalidPackageSnapshotTime, snapshotTime, err)
	}

	return nil
}
