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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
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
	ErrUnsupportedFedoraFeature       = NewImageCustomizerError("Validation:UnsupportedFedoraFeature", "unsupported feature for Fedora images")
)

func ValidateConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config,
	newImage bool, options ImageCustomizerOptions,
) (*ResolvedConfig, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "validate_config")
	defer span.End()

	rc := &ResolvedConfig{
		BaseConfigPath: baseConfigPath,
		Config:         config,
		Options:        options,
	}

	err := options.IsValid()
	if err != nil {
		return nil, err
	}

	err = config.IsValid()
	if err != nil {
		return nil, err
	}

	rc.ConfigChain, err = buildConfigChain(ctx, rc)
	if err != nil {
		return nil, err
	}

	rc.CustomizeOSPartitions = config.CustomizePartitions() ||
		config.OS != nil ||
		len(config.Scripts.PostCustomization) > 0 ||
		len(config.Scripts.FinalizeCustomization) > 0

	// Create a UUID for the image.
	rc.ImageUuid, rc.ImageUuidStr, err = randomization.CreateUuid()
	if err != nil {
		return nil, err
	}

	// Resolve build dir path.
	rc.BuildDirAbs, err = filepath.Abs(options.BuildDir)
	if err != nil {
		return nil, err
	}

	// Intermediate writeable image
	rc.RawImageFile = filepath.Join(rc.BuildDirAbs, BaseImageName)

	err = ValidateRpmSources(options.RpmsSources)
	if err != nil {
		return nil, err
	}

	if !newImage {
		rc.InputImageFile, rc.InputImageOci, err = validateInput(rc.ConfigChain, options.InputImageFile)
		if err != nil {
			return nil, err
		}
	}

	err = validateIsoConfig(baseConfigPath, config.Iso)
	if err != nil {
		return nil, err
	}

	err = validateOsConfig(baseConfigPath, config.OS, options.RpmsSources, options.UseBaseImageRpmRepos)
	if err != nil {
		return nil, err
	}

	rc.Config.OS.Hostname = resolveHostname(rc.ConfigChain)

	err = validateScripts(baseConfigPath, &config.Scripts)
	if err != nil {
		return nil, err
	}

	rc.OutputImageFormat, err = validateOutputImageFormat(rc.ConfigChain, options.OutputImageFormat)
	if err != nil {
		return nil, err
	}

	rc.OutputImageFile, err = validateOutputImageFile(rc.ConfigChain, options.OutputImageFile, rc.OutputImageFormat)
	if err != nil {
		return nil, err
	}

	rc.OutputArtifacts = resolveOutputArtifacts(rc.ConfigChain)

	rc.PackageSnapshotTime, err = validatePackageSnapshotTime(options.PackageSnapshotTime, config)
	if err != nil {
		return nil, err
	}

	return rc, nil
}

func ValidateConfigPostImageDownload(rc *ResolvedConfig) error {
	err := validateIsoPxeCustomization(rc)
	if err != nil {
		return err
	}

	return nil
}

func validateInput(configChain []*ConfigWithBasePath, inputImageFile string,
) (string, *imagecustomizerapi.OciImage, error) {
	if inputImageFile != "" {
		if yes, err := file.IsFile(inputImageFile); err != nil {
			return "", nil, fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidInputImageFileArg, inputImageFile, err)
		} else if !yes {
			return "", nil, fmt.Errorf("%w (file='%s')", ErrInputImageFileNotFile, inputImageFile)
		}
		return inputImageFile, nil, nil
	}

	// Resolve input image path
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Input.Image.Path != "" {
			inputImageAbsPath := file.GetAbsPathWithBase(
				configWithBase.BaseConfigPath,
				configWithBase.Config.Input.Image.Path,
			)

			// Validate the path
			if yes, err := file.IsFile(inputImageAbsPath); err != nil {
				return "", nil, fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidInputImageFileConfig, configWithBase.Config.Input.Image.Path, err)
			} else if !yes {
				return "", nil, fmt.Errorf("%w (path='%s')", ErrInputImageFileNotFile, configWithBase.Config.Input.Image.Path)
			}

			return inputImageAbsPath, nil, nil
		}

		if configWithBase.Config.Input.Image.Oci != nil {
			return "", configWithBase.Config.Input.Image.Oci, nil
		}
	}

	return "", nil, ErrInputImageFileRequired
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

func validateOutputImageFormat(configChain []*ConfigWithBasePath, cliOutputImageFormat imagecustomizerapi.ImageFormatType,
) (imagecustomizerapi.ImageFormatType, error) {
	if cliOutputImageFormat != "" {
		return cliOutputImageFormat, nil
	}

	// Resolve output image format
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Output.Image.Format != "" {
			return configWithBase.Config.Output.Image.Format, nil
		}
	}

	return "", ErrOutputImageFormatRequired
}

func validateOutputImageFile(configChain []*ConfigWithBasePath, cliOutputImageFile string,
	outputImageFormat imagecustomizerapi.ImageFormatType,
) (string, error) {
	if cliOutputImageFile != "" {
		if outputImageFormat != imagecustomizerapi.ImageFormatTypePxeDir {
			if isDir, err := file.DirExists(cliOutputImageFile); err != nil {
				return "", fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidOutputImageFileArg, cliOutputImageFile, err)
			} else if isDir {
				return "", fmt.Errorf("%w (file='%s')", ErrOutputImageFileIsDirectory, cliOutputImageFile)
			}
		}
		return cliOutputImageFile, nil
	}

	// Resolve output image path
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Output.Image.Path != "" {
			outputImageFile := file.GetAbsPathWithBase(
				configWithBase.BaseConfigPath,
				configWithBase.Config.Output.Image.Path,
			)

			// PXE output format allows the output to be a directory
			if outputImageFormat != imagecustomizerapi.ImageFormatTypePxeDir {
				if isDir, err := file.DirExists(outputImageFile); err != nil {
					return "", fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidOutputImageFileConfig,
						configWithBase.Config.Output.Image.Path, err)
				} else if isDir {
					return "", fmt.Errorf("%w (file='%s')", ErrOutputImageFileIsDirectory,
						configWithBase.Config.Output.Image.Path)
				}
			}

			return outputImageFile, nil
		}
	}

	return "", ErrOutputImageFileRequired
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

func validatePackageSnapshotTime(cliSnapshotTime imagecustomizerapi.PackageSnapshotTime,
	config *imagecustomizerapi.Config,
) (imagecustomizerapi.PackageSnapshotTime, error) {
	snapshotTime := imagecustomizerapi.PackageSnapshotTime("")
	switch {
	case cliSnapshotTime != "":
		snapshotTime = cliSnapshotTime

	case config.OS != nil && config.OS.Packages.SnapshotTime != "":
		snapshotTime = config.OS.Packages.SnapshotTime
	}

	if snapshotTime != "" {
		if !slices.Contains(config.PreviewFeatures, imagecustomizerapi.PreviewFeaturePackageSnapshotTime) {
			return "", ErrPackageSnapshotPreviewRequired
		}
	}

	return snapshotTime, nil
}

func validateIsoPxeCustomization(rc *ResolvedConfig) error {
	if rc.InputIsIso() {
		// While re-creating a disk image from the iso is technically possible,
		// we are choosing to not implement it until there is a need.
		if !rc.OutputIsIso() && !rc.OutputIsPxe() {
			return fmt.Errorf("%w (output='%s', input='%s')", ErrCannotGenerateOutputFormat, rc.OutputImageFormat,
				rc.InputFileExt())
		}

		// While defining a storage configuration can work when the input image is
		// an iso, there is no obvious point of moving content between partitions
		// where all partitions get collapsed into the squashfs at the end.
		if rc.Config.CustomizePartitions() {
			return ErrCannotCustomizePartitionsOnIso
		}
	}

	return nil
}

func resolveOutputArtifacts(configChain []*ConfigWithBasePath) *imagecustomizerapi.Artifacts {
	var artifacts *imagecustomizerapi.Artifacts

	for _, configWithBase := range configChain {
		if configWithBase.Config.Output.Artifacts != nil {
			if artifacts == nil {
				artifacts = &imagecustomizerapi.Artifacts{}
			}

			// Artifacts path from current config overrides previous one
			if configWithBase.Config.Output.Artifacts.Path != "" {
				artifacts.Path = file.GetAbsPathWithBase(
					configWithBase.BaseConfigPath,
					configWithBase.Config.Output.Artifacts.Path,
				)
			}

			// Append items
			artifacts.Items = mergeOutputArtifactTypes(
				artifacts.Items,
				configWithBase.Config.Output.Artifacts.Items,
			)
		}
	}

	return artifacts
}

func mergeOutputArtifactTypes(base, current []imagecustomizerapi.OutputArtifactsItemType,
) []imagecustomizerapi.OutputArtifactsItemType {
	seen := make(map[imagecustomizerapi.OutputArtifactsItemType]bool)
	var merged []imagecustomizerapi.OutputArtifactsItemType

	// Add base items first
	for _, item := range base {
		if !seen[item] {
			merged = append(merged, item)
			seen[item] = true
		}
	}

	// Add current items
	for _, item := range current {
		if !seen[item] {
			merged = append(merged, item)
			seen[item] = true
		}
	}

	return merged
}

func resolveHostname(configChain []*ConfigWithBasePath) string {
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.OS != nil && configWithBase.Config.OS.Hostname != "" {
			return configWithBase.Config.OS.Hostname
		}
	}

	return ""
}
