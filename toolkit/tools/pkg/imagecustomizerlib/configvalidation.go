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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// Validation errors
	ErrInputImageFileRequired                = NewImageCustomizerError("Validation:InputImageFileRequired", "input image file must be specified")
	ErrInvalidInputImageFileArg              = NewImageCustomizerError("Validation:InvalidInputImageFileArg", "invalid command-line option '--image-file'")
	ErrInputImageFileNotFile                 = NewImageCustomizerError("Validation:InputImageFileNotFile", "input image file is not a file")
	ErrInvalidInputImageFileConfig           = NewImageCustomizerError("Validation:InvalidInputImageFileConfig", "invalid config file property 'input.image.path'")
	ErrInvalidAdditionalFilesSource          = NewImageCustomizerError("Validation:InvalidAdditionalFilesSource", "invalid additionalFiles source file")
	ErrAdditionalFilesSourceNotFile          = NewImageCustomizerError("Validation:AdditionalFilesSourceNotFile", "additionalFiles source file is not a file")
	ErrInvalidPostCustomizationScript        = NewImageCustomizerError("Validation:InvalidPostCustomizationScript", "invalid postCustomization script")
	ErrInvalidFinalizeScript                 = NewImageCustomizerError("Validation:InvalidFinalizeScript", "invalid finalizeCustomization script")
	ErrScriptNotUnderConfigDir               = NewImageCustomizerError("Validation:ScriptNotUnderConfigDir", "script file is not under config directory")
	ErrScriptFileNotReadable                 = NewImageCustomizerError("Validation:ScriptFileNotReadable", "couldn't read script file")
	ErrScriptFileNotFile                     = NewImageCustomizerError("Validation:ScriptFileNotFile", "script file is not a file")
	ErrNoRpmSourcesSpecified                 = NewImageCustomizerError("Validation:NoRpmSourcesSpecified", "have packages to install or update but no RPM sources were specified")
	ErrOutputImageFileRequired               = NewImageCustomizerError("Validation:OutputImageFileRequired", "output image file must be specified")
	ErrInvalidOutputImageFileArg             = NewImageCustomizerError("Validation:InvalidOutputImageFileArg", "invalid command-line option '--output-image-file'")
	ErrOutputImageFileIsDirectory            = NewImageCustomizerError("Validation:OutputImageFileIsDirectory", "output image file is a directory")
	ErrInvalidOutputImageFileConfig          = NewImageCustomizerError("Validation:InvalidOutputImageFileConfig", "invalid config file property 'output.image.path'")
	ErrOutputImageFormatRequired             = NewImageCustomizerError("Validation:OutputImageFormatRequired", "output image format must be specified")
	ErrInvalidUser                           = NewImageCustomizerError("Validation:InvalidUser", "invalid user")
	ErrInvalidSSHPublicKeyFile               = NewImageCustomizerError("Validation:InvalidSSHPublicKeyFile", "failed to find SSH public key file")
	ErrSSHPublicKeyNotFile                   = NewImageCustomizerError("Validation:SSHPublicKeyNotFile", "SSH public key path is not a file")
	ErrInvalidPasswordFile                   = NewImageCustomizerError("Validation:InvalidPasswordFile", "failed to find password file")
	ErrPasswordFileNotFile                   = NewImageCustomizerError("Validation:PasswordFileNotFile", "password file is not a file")
	ErrAdditionalDirsSourceNotReadable       = NewImageCustomizerError("Validation:AdditionalDirsSourceNotReadable", "failed to read additionalDirs source")
	ErrAdditionalDirsSourceIsFile            = NewImageCustomizerError("Validation:AdditionalDirsSourceIsFile", "additionalDirs source exists but is a file")
	ErrAdditionalDirsSourceNotFound          = NewImageCustomizerError("Validation:AdditionalDirsSourceNotFound", "additionalDirs source does not exist")
	ErrInvalidPackageSnapshotTime            = NewImageCustomizerError("Validation:InvalidPackageSnapshotTime", "invalid command-line option '--package-snapshot-time'")
	ErrUnsupportedFedoraFeature              = NewImageCustomizerError("Validation:UnsupportedFedoraFeature", "unsupported feature for Fedora images")
	ErrInvalidOutputSelinuxPolicyPathArg     = NewImageCustomizerError("Validation:InvalidOutputSelinuxPolicyPathArg", "invalid command-line option '--output-selinux-policy-path'")
	ErrOutputSelinuxPolicyPathIsFileArg      = NewImageCustomizerError("Validation:OutputSelinuxPolicyPathIsFileArg", "path exists but is a file")
	ErrOutputSelinuxPolicyPathNotFoundArg    = NewImageCustomizerError("Validation:OutputSelinuxPolicyPathNotDirArg", "path does not exist")
	ErrInvalidOutputSelinuxPolicyPathConfig  = NewImageCustomizerError("Validation:InvalidOutputSelinuxPolicyPathConfig", "invalid config file property 'output.selinuxPolicyPath'")
	ErrOutputSelinuxPolicyPathIsFileConfig   = NewImageCustomizerError("Validation:OutputSelinuxPolicyPathIsFileConfig", "path exists but is a file")
	ErrOutputSelinuxPolicyPathNotFoundConfig = NewImageCustomizerError("Validation:OutputSelinuxPolicyPathNotDirConfig", "path does not exist")
	ErrInvalidInputImageAzureLinux           = NewImageCustomizerError("Validation:InvalidInputImageAzureLinux", "invalid input.image.azureLinux config")
	ErrInputImageAzureLinuxNotFound          = NewImageCustomizerError("Validation:InputImageAzureLinuxNotFound", "input.image.azureLinux not found")
	ErrInputImageOciNotFound                 = NewImageCustomizerError("Validation:InputImageOciNotFound", "input.image.oci not found")
)

// ValidateConfigWithConfigFileOptions validates a configuration file without performing customization.
// This function does not require root permissions and can be used to validate config files
// before running the actual customization process.
func ValidateConfigWithConfigFileOptions(ctx context.Context, configFile string, options ValidateConfigOptions,
) (err error) {
	var config imagecustomizerapi.Config

	baseConfigPath, _ := filepath.Split(configFile)

	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrGetAbsoluteConfigPath, err)
	}

	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	ctx, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "validate_config_command")
	span.SetAttributes(
		attribute.Bool("validate_resources_files", options.ValidateResources.ValidateFiles()),
		attribute.Bool("validate_resources_oci", options.ValidateResources.ValidateOci()),
		attribute.Bool("validate_resources_all",
			options.ValidateResources.Contains(imagecustomizerapi.ValidateResourceTypeAll)),
	)
	defer finishSpanWithError(span, &err)

	err = options.IsValid()
	if err != nil {
		return err
	}

	customizeOptions := ImageCustomizerOptions{
		// BuildDir is required when validating Azure Linux (OCI) input images for signature verification to
		// create a temporary notary trust store.
		BuildDir: options.BuildDir,
	}

	// Pass newImage=false to emulate validation during an actual customization run.
	// Pass allowPartialConfig=true to allow configs that omit input/output fields (provided via CLI during customize).
	_, err = ValidateConfig(ctx, absBaseConfigPath, &config, false, true, options.ValidateResources, customizeOptions)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidImageConfig, err)
	}

	logger.Log.Infof("Config validation succeeded")

	return nil
}

// ValidateConfig validates the configuration and returns a resolved configuration.
// options.BuildDir must be non-empty if newImage=false and the resolved input image may be an OCI Azure Linux image.
// The directory will be created if it doesn't exist. ResolvedConfig.BuildDirAbs will be set to its absolute path.
func ValidateConfig(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.Config,
	newImage bool, allowPartialConfig bool, validateResources imagecustomizerapi.ValidateResourceTypes,
	options ImageCustomizerOptions,
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

	err = options.verifyPreviewFeatures(config.PreviewFeatures)
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
	if options.BuildDir != "" {
		rc.BuildDirAbs, err = filepath.Abs(options.BuildDir)
		if err != nil {
			return nil, err
		}

		// Ensure build fold dir exists.
		err = os.MkdirAll(rc.BuildDirAbs, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	// Intermediate writeable image
	rc.RawImageFile = filepath.Join(rc.BuildDirAbs, BaseImageName)

	err = ValidateRpmSources(options.RpmsSources)
	if err != nil {
		return nil, err
	}

	validateFiles := validateResources.ValidateFiles()
	validateOci := validateResources.ValidateOci()

	if !newImage {
		rc.InputImage, rc.InputImageOciDescriptor, err = validateInput(ctx, rc.BuildDirAbs, rc.ConfigChain,
			options.InputImageFile, options.InputImage, validateFiles, validateOci, allowPartialConfig)
		if err != nil {
			return nil, err
		}
	}

	err = validateIsoConfigChain(rc.ConfigChain, validateFiles)
	if err != nil {
		return nil, err
	}

	rc.Iso = resolveIsoConfig(rc.ConfigChain)

	err = validatePxeConfigChain(rc.ConfigChain, validateFiles)
	if err != nil {
		return nil, err
	}

	rc.Pxe = resolvePxeConfig(rc.ConfigChain)

	err = validateOsConfig(baseConfigPath, config.OS, options.RpmsSources, options.UseBaseImageRpmRepos, validateFiles,
		allowPartialConfig)
	if err != nil {
		return nil, err
	}

	rc.Hostname = resolveHostname(rc.ConfigChain)
	rc.SELinux = resolveSeLinux(rc.ConfigChain)
	rc.BootLoader.ResetType = resolveBootLoaderResetType(rc.ConfigChain)
	rc.Uki = resolveUki(rc.ConfigChain)
	rc.OsKernelCommandLine = resolveOsKernelCommandLine(rc.ConfigChain)

	err = validateScripts(baseConfigPath, &config.Scripts, validateFiles)
	if err != nil {
		return nil, err
	}

	rc.OutputImageFormat, err = validateOutputImageFormat(rc.ConfigChain, options.OutputImageFormat, allowPartialConfig)
	if err != nil {
		return nil, err
	}

	rc.OutputImageFile, err = validateOutputImageFile(rc.ConfigChain, options.OutputImageFile, rc.OutputImageFormat,
		validateFiles, allowPartialConfig)
	if err != nil {
		return nil, err
	}

	rc.OutputArtifacts = resolveOutputArtifacts(rc.ConfigChain)

	rc.OutputSelinuxPolicyPath, err = validateOutputSelinuxPolicyPath(rc.ConfigChain, options.OutputSelinuxPolicyPath,
		validateFiles)
	if err != nil {
		return nil, err
	}

	rc.CosiCompressionLevel = resolveCosiCompressionLevel(rc.ConfigChain, options.CosiCompressionLevel,
		rc.OutputImageFormat)
	rc.CosiCompressionLong = defaultCosiCompressionLong(rc.OutputImageFormat)

	return rc, nil
}

func ValidateConfigPostImageDownload(rc *ResolvedConfig) error {
	err := validateIsoPxeCustomization(rc)
	if err != nil {
		return err
	}

	return nil
}

// validateInput validates the input image configuration and returns the resolved input image.
// buildDir must exist and be a writable directory if the resolved image is an OCI Azure Linux image.
func validateInput(ctx context.Context, buildDir string, configChain []*ConfigWithBasePath, inputImageFile string,
	inputImage string, validateFiles bool, validateOci bool, allowPartialConfig bool,
) (imagecustomizerapi.InputImage, *ociv1.Descriptor, error) {
	if inputImageFile != "" {
		if validateFiles {
			if yes, err := file.IsFile(inputImageFile); err != nil {
				err = fmt.Errorf("%w (file='%s'):\n%w", ErrInvalidInputImageFileArg, inputImageFile, err)
				return imagecustomizerapi.InputImage{}, nil, err
			} else if !yes {
				err = fmt.Errorf("%w (file='%s')", ErrInputImageFileNotFile, inputImageFile)
				return imagecustomizerapi.InputImage{}, nil, err
			}
		}

		return imagecustomizerapi.InputImage{
			Path: inputImageFile,
		}, nil, nil
	}

	if inputImage != "" {
		inputImage, err := parseInputImage(inputImage)
		if err != nil {
			return imagecustomizerapi.InputImage{}, nil, err
		}

		return inputImage, nil, nil
	}

	// Resolve input image path
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Input.Image.Path != "" {
			inputImageAbsPath := file.GetAbsPathWithBase(
				configWithBase.BaseConfigPath,
				configWithBase.Config.Input.Image.Path,
			)

			if validateFiles {
				if yes, err := file.IsFile(inputImageAbsPath); err != nil {
					err = fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidInputImageFileConfig,
						configWithBase.Config.Input.Image.Path, err)
					return imagecustomizerapi.InputImage{}, nil, err
				} else if !yes {
					err = fmt.Errorf("%w (path='%s')", ErrInputImageFileNotFile, configWithBase.Config.Input.Image.Path)
					return imagecustomizerapi.InputImage{}, nil, err
				}
			}

			return imagecustomizerapi.InputImage{
				Path: inputImageAbsPath,
			}, nil, nil
		}

		if configWithBase.Config.Input.Image.Oci != nil {
			var descriptor *ociv1.Descriptor
			if validateOci {
				_, desc, err := openOciImage(ctx, *configWithBase.Config.Input.Image.Oci, nil, nil, "")
				if err != nil {
					err = fmt.Errorf("%w (uri='%s'):\n%w", ErrInputImageOciNotFound,
						configWithBase.Config.Input.Image.Oci.Uri, err)
					return imagecustomizerapi.InputImage{}, nil, err
				}
				descriptor = &desc
			}

			return imagecustomizerapi.InputImage{
				Oci: configWithBase.Config.Input.Image.Oci,
			}, descriptor, nil
		}

		if configWithBase.Config.Input.Image.AzureLinux != nil {
			var descriptor *ociv1.Descriptor
			if validateOci {
				ociImage, err := generateAzureLinuxOciUri(*configWithBase.Config.Input.Image.AzureLinux)
				if err != nil {
					return imagecustomizerapi.InputImage{}, nil, fmt.Errorf("%w (variant='%s', version='%s'):\n%w",
						ErrInvalidInputImageAzureLinux, configWithBase.Config.Input.Image.AzureLinux.Variant,
						configWithBase.Config.Input.Image.AzureLinux.Version, err)
				}

				signatureCheckOptions := getAzureLinuxOciSignatureCheckOptions()
				_, desc, err := openOciImage(ctx, ociImage, nil, signatureCheckOptions, buildDir)
				if err != nil {
					err := fmt.Errorf("%w (variant='%s', version='%s'):\n%w", ErrInputImageAzureLinuxNotFound,
						configWithBase.Config.Input.Image.AzureLinux.Variant,
						configWithBase.Config.Input.Image.AzureLinux.Version, err)
					return imagecustomizerapi.InputImage{}, nil, err
				}
				descriptor = &desc
			}

			return imagecustomizerapi.InputImage{
				AzureLinux: configWithBase.Config.Input.Image.AzureLinux,
			}, descriptor, nil
		}
	}

	if allowPartialConfig {
		return imagecustomizerapi.InputImage{}, nil, nil
	}
	return imagecustomizerapi.InputImage{}, nil, ErrInputImageFileRequired
}

func validateAdditionalFiles(baseConfigPath string, additionalFiles imagecustomizerapi.AdditionalFileList,
	validateFiles bool,
) error {
	if !validateFiles {
		return nil
	}

	errs := []error(nil)
	for _, additionalFile := range additionalFiles {
		switch {
		case additionalFile.Source != "":
			sourceFileFullPath := file.GetAbsPathWithBase(baseConfigPath, additionalFile.Source)
			if yes, err := file.IsFile(sourceFileFullPath); err != nil {
				errs = append(errs, fmt.Errorf("%w (source='%s'):\n%w", ErrInvalidAdditionalFilesSource, additionalFile.Source, err))
			} else if !yes {
				errs = append(errs, fmt.Errorf("%w (source='%s')", ErrAdditionalFilesSourceNotFile,
					additionalFile.Source))
			}
		}
	}

	return errors.Join(errs...)
}

func validateAdditionalDirs(baseConfigPath string, additionalDirs imagecustomizerapi.DirConfigList,
	validateFiles bool,
) error {
	if !validateFiles {
		return nil
	}

	errs := []error(nil)
	for _, additionalDir := range additionalDirs {
		if additionalDir.Source != "" {
			sourceDirFullPath := file.GetAbsPathWithBase(baseConfigPath, additionalDir.Source)
			if isFile, err := file.IsFile(sourceDirFullPath); err != nil {
				if os.IsNotExist(err) {
					errs = append(errs,
						fmt.Errorf("%w (source='%s')", ErrAdditionalDirsSourceNotFound, additionalDir.Source))
				} else {
					errs = append(errs,
						fmt.Errorf("%w (source='%s'):\n%w", ErrAdditionalDirsSourceNotReadable, additionalDir.Source, err))
				}
			} else if isFile {
				errs = append(errs,
					fmt.Errorf("%w (source='%s')", ErrAdditionalDirsSourceIsFile, additionalDir.Source))
			}
		}
	}

	return errors.Join(errs...)
}

func validateIsoConfig(baseConfigPath string, config *imagecustomizerapi.Iso, validateFiles bool) error {
	if config == nil {
		return nil
	}

	err := validateAdditionalFiles(baseConfigPath, config.AdditionalFiles, validateFiles)
	if err != nil {
		return err
	}

	return nil
}

func validateIsoConfigChain(configChain []*ConfigWithBasePath, validateFiles bool) error {
	for _, configWithBase := range configChain {
		err := validateIsoConfig(configWithBase.BaseConfigPath, configWithBase.Config.Iso, validateFiles)
		if err != nil {
			return fmt.Errorf("invalid 'iso' config:\n%w", err)
		}
	}
	return nil
}

func validatePxeConfig(baseConfigPath string, config *imagecustomizerapi.Pxe, validateFiles bool) error {
	if config == nil {
		return nil
	}

	err := validateAdditionalFiles(baseConfigPath, config.AdditionalFiles, validateFiles)
	if err != nil {
		return err
	}

	return nil
}

func validatePxeConfigChain(configChain []*ConfigWithBasePath, validateFiles bool) error {
	for _, configWithBase := range configChain {
		err := validatePxeConfig(configWithBase.BaseConfigPath, configWithBase.Config.Pxe, validateFiles)
		if err != nil {
			return fmt.Errorf("invalid 'pxe' config:\n%w", err)
		}
	}
	return nil
}

func validateOsConfig(baseConfigPath string, config *imagecustomizerapi.OS, rpmsSources []string,
	useBaseImageRpmRepos bool, validateFiles bool, allowPartialConfig bool,
) error {
	if config == nil {
		return nil
	}

	var err error

	err = validatePackageLists(baseConfigPath, config, rpmsSources, useBaseImageRpmRepos, allowPartialConfig)
	if err != nil {
		return err
	}

	err = validateAdditionalFiles(baseConfigPath, config.AdditionalFiles, validateFiles)
	if err != nil {
		return err
	}

	err = validateAdditionalDirs(baseConfigPath, config.AdditionalDirs, validateFiles)
	if err != nil {
		return err
	}

	err = validateUsers(baseConfigPath, config.Users, validateFiles)
	if err != nil {
		return err
	}

	return nil
}

func validateScripts(baseConfigPath string, scripts *imagecustomizerapi.Scripts, validateFiles bool) error {
	if scripts == nil {
		return nil
	}

	for i, script := range scripts.PostCustomization {
		err := validateScript(baseConfigPath, &script, validateFiles)
		if err != nil {
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidPostCustomizationScript, i, err)
		}
	}

	for i, script := range scripts.FinalizeCustomization {
		err := validateScript(baseConfigPath, &script, validateFiles)
		if err != nil {
			return fmt.Errorf("%w (index=%d):\n%w", ErrInvalidFinalizeScript, i, err)
		}
	}

	return nil
}

func validateScript(baseConfigPath string, script *imagecustomizerapi.Script, validateFiles bool) error {
	if script.Path != "" {
		// Ensure that install scripts sit under the config file's parent directory.
		// This allows the install script to be run in the chroot environment by bind mounting the config directory.
		if !filepath.IsLocal(script.Path) {
			return fmt.Errorf("%w (script='%s', config='%s')", ErrScriptNotUnderConfigDir, script.Path, baseConfigPath)
		}

		if validateFiles {
			fullPath := filepath.Join(baseConfigPath, script.Path)
			if isFile, err := file.IsFile(fullPath); err != nil {
				return fmt.Errorf("%w (script='%s'):\n%w", ErrScriptFileNotReadable, script.Path, err)
			} else if !isFile {
				return fmt.Errorf("%w (script='%s')", ErrScriptFileNotFile, script.Path)
			}
		}
	}

	return nil
}

func validatePackageLists(baseConfigPath string, config *imagecustomizerapi.OS, rpmsSources []string,
	useBaseImageRpmRepos bool, allowPartialConfig bool,
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

	needRpmsSources := len(allPackagesInstall) > 0 || len(allPackagesUpdate) > 0 ||
		config.Packages.UpdateExistingPackages
	hasRpmSources := len(rpmsSources) > 0 || useBaseImageRpmRepos

	if needRpmsSources && !hasRpmSources && !allowPartialConfig {
		return ErrNoRpmSourcesSpecified
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
	allowPartialConfig bool,
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

	if allowPartialConfig {
		return "", nil
	}
	return "", ErrOutputImageFormatRequired
}

func validateOutputImageFile(configChain []*ConfigWithBasePath, cliOutputImageFile string,
	outputImageFormat imagecustomizerapi.ImageFormatType, validateFiles bool, allowPartialConfig bool,
) (string, error) {
	if cliOutputImageFile != "" {
		if validateFiles && outputImageFormat != imagecustomizerapi.ImageFormatTypePxeDir {
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
			if validateFiles && outputImageFormat != imagecustomizerapi.ImageFormatTypePxeDir {
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

	if allowPartialConfig {
		return "", nil
	}
	return "", ErrOutputImageFileRequired
}

func validateUsers(baseConfigPath string, users []imagecustomizerapi.User, validateFiles bool) error {
	for _, user := range users {
		err := validateUser(baseConfigPath, user, validateFiles)
		if err != nil {
			return fmt.Errorf("%w (user='%s'):\n%w", ErrInvalidUser, user.Name, err)
		}
	}

	return nil
}

func validateUser(baseConfigPath string, user imagecustomizerapi.User, validateFiles bool) error {
	if !validateFiles {
		return nil
	}

	for _, path := range user.SSHPublicKeyPaths {
		absPath := file.GetAbsPathWithBase(baseConfigPath, path)
		if isFile, err := file.IsFile(absPath); err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidSSHPublicKeyFile, path, err)
		} else if !isFile {
			return fmt.Errorf("%w (path='%s')", ErrSSHPublicKeyNotFile, path)
		}
	}

	// Validate password file if type is plain-text-file or hashed-file
	if user.Password != nil &&
		(user.Password.Type == imagecustomizerapi.PasswordTypePlainTextFile ||
			user.Password.Type == imagecustomizerapi.PasswordTypeHashedFile) {
		absPath := file.GetAbsPathWithBase(baseConfigPath, user.Password.Value)
		if isFile, err := file.IsFile(absPath); err != nil {
			return fmt.Errorf("%w (value='%s'):\n%w", ErrInvalidPasswordFile, user.Password.Value, err)
		} else if !isFile {
			return fmt.Errorf("%w (value='%s')", ErrPasswordFileNotFile, user.Password.Value)
		}
	}

	return nil
}

func validateOutputSelinuxPolicyPath(configChain []*ConfigWithBasePath, cliOutputSelinuxPolicyPath string,
	validateFiles bool,
) (string, error) {
	// CLI parameter takes precedence.
	if cliOutputSelinuxPolicyPath != "" {
		if validateFiles {
			if isFile, err := file.IsFile(cliOutputSelinuxPolicyPath); err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("%w (path='%s')", ErrOutputSelinuxPolicyPathNotFoundArg,
						cliOutputSelinuxPolicyPath)
				}
				return "", fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidOutputSelinuxPolicyPathArg,
					cliOutputSelinuxPolicyPath, err)
			} else if isFile {
				return "", fmt.Errorf("%w (path='%s')", ErrOutputSelinuxPolicyPathIsFileArg, cliOutputSelinuxPolicyPath)
			}
		}
		return cliOutputSelinuxPolicyPath, nil
	}

	// Resolve from config chain.
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Output.SelinuxPolicyPath != "" {
			outputSelinuxPolicyPath := file.GetAbsPathWithBase(
				configWithBase.BaseConfigPath,
				configWithBase.Config.Output.SelinuxPolicyPath,
			)

			if validateFiles {
				if isFile, err := file.IsFile(outputSelinuxPolicyPath); err != nil {
					if os.IsNotExist(err) {
						return "", fmt.Errorf("%w (path='%s')", ErrOutputSelinuxPolicyPathNotFoundConfig,
							configWithBase.Config.Output.SelinuxPolicyPath)
					}
					return "", fmt.Errorf("%w (path='%s'):\n%w", ErrInvalidOutputSelinuxPolicyPathConfig,
						configWithBase.Config.Output.SelinuxPolicyPath, err)
				} else if isFile {
					return "", fmt.Errorf("%w (path='%s')", ErrOutputSelinuxPolicyPathIsFileConfig,
						configWithBase.Config.Output.SelinuxPolicyPath)
				}
			}

			return outputSelinuxPolicyPath, nil
		}
	}

	// Empty string is valid, the feature is optional.
	return "", nil
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

func resolveSeLinux(configChain []*ConfigWithBasePath) imagecustomizerapi.SELinux {
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.OS != nil && configWithBase.Config.OS.SELinux.Mode != "" {
			return configWithBase.Config.OS.SELinux
		}
	}
	return imagecustomizerapi.SELinux{}
}

func resolveUki(configChain []*ConfigWithBasePath) *imagecustomizerapi.Uki {
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.OS != nil && configWithBase.Config.OS.Uki != nil {
			return configWithBase.Config.OS.Uki
		}
	}
	return nil
}

func resolveBootLoaderResetType(configChain []*ConfigWithBasePath) imagecustomizerapi.ResetBootLoaderType {
	for _, cfg := range slices.Backward(configChain) {
		if cfg.Config.OS == nil {
			continue
		}

		switch cfg.Config.OS.BootLoader.ResetType {
		case imagecustomizerapi.ResetBootLoaderTypeHard:
			return imagecustomizerapi.ResetBootLoaderTypeHard
		case "":
			// skip unset, keep searching
			continue
		default:
			continue
		}
	}
	return ""
}

// mergeKernelCommandLine merges kernel command line arguments from a config chain.
// The getArgs function extracts the ExtraCommandLine slice from each config.
func mergeKernelCommandLine(configChain []*ConfigWithBasePath,
	getArgs func(*imagecustomizerapi.Config) []string,
) []string {
	var mergedArgs []string
	for _, configWithBase := range configChain {
		args := getArgs(configWithBase.Config)
		if len(args) > 0 {
			mergedArgs = append(mergedArgs, args...)
		}
	}
	return mergedArgs
}

// mergeAdditionalFiles merges additional files from a config chain, resolving source paths.
// The getFiles function extracts the AdditionalFileList from each config.
func mergeAdditionalFiles(configChain []*ConfigWithBasePath,
	getFiles func(*imagecustomizerapi.Config) imagecustomizerapi.AdditionalFileList,
) imagecustomizerapi.AdditionalFileList {
	var merged imagecustomizerapi.AdditionalFileList
	for _, configWithBase := range configChain {
		files := getFiles(configWithBase.Config)
		for _, additionalFile := range files {
			resolvedFile := additionalFile
			if additionalFile.Source != "" {
				resolvedFile.Source = file.GetAbsPathWithBase(configWithBase.BaseConfigPath, additionalFile.Source)
			}
			merged = append(merged, resolvedFile)
		}
	}
	return merged
}

func resolveOsKernelCommandLine(configChain []*ConfigWithBasePath) imagecustomizerapi.KernelCommandLine {
	mergedArgs := mergeKernelCommandLine(configChain, func(c *imagecustomizerapi.Config) []string {
		if c.OS != nil {
			return c.OS.KernelCommandLine.ExtraCommandLine
		}
		return nil
	})

	return imagecustomizerapi.KernelCommandLine{
		ExtraCommandLine: mergedArgs,
	}
}

// resolveIsoConfig builds a resolved Iso config by merging values from the config chain.
// AdditionalFiles and KernelCommandLine are merged (concatenated from all configs).
// InitramfsType and KdumpBootFiles use "current overrides base" semantics.
func resolveIsoConfig(configChain []*ConfigWithBasePath) imagecustomizerapi.Iso {
	var iso imagecustomizerapi.Iso

	// Merge AdditionalFiles (concatenate from all configs)
	iso.AdditionalFiles = mergeAdditionalFiles(configChain,
		func(c *imagecustomizerapi.Config) imagecustomizerapi.AdditionalFileList {
			if c.Iso != nil {
				return c.Iso.AdditionalFiles
			}
			return nil
		})

	// Merge KernelCommandLine (concatenate from all configs)
	iso.KernelCommandLine.ExtraCommandLine = mergeKernelCommandLine(configChain,
		func(c *imagecustomizerapi.Config) []string {
			if c.Iso != nil {
				return c.Iso.KernelCommandLine.ExtraCommandLine
			}
			return nil
		})

	// Resolve InitramfsType (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Iso != nil &&
			configWithBase.Config.Iso.InitramfsType != "" {
			iso.InitramfsType = configWithBase.Config.Iso.InitramfsType
			break
		}
	}

	// Resolve KdumpBootFiles (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Iso != nil &&
			configWithBase.Config.Iso.KdumpBootFiles != nil {
			iso.KdumpBootFiles = configWithBase.Config.Iso.KdumpBootFiles
			break
		}
	}

	return iso
}

// resolvePxeConfig builds a resolved Pxe config by merging values from the config chain.
// AdditionalFiles and KernelCommandLine are merged (concatenated from all configs).
// InitramfsType, KdumpBootFiles, BootstrapBaseUrl, and BootstrapFileUrl use "current overrides base" semantics.
func resolvePxeConfig(configChain []*ConfigWithBasePath) imagecustomizerapi.Pxe {
	var pxe imagecustomizerapi.Pxe

	// Merge AdditionalFiles (concatenate from all configs)
	pxe.AdditionalFiles = mergeAdditionalFiles(configChain,
		func(c *imagecustomizerapi.Config) imagecustomizerapi.AdditionalFileList {
			if c.Pxe != nil {
				return c.Pxe.AdditionalFiles
			}
			return nil
		})

	// Merge KernelCommandLine (concatenate from all configs)
	pxe.KernelCommandLine.ExtraCommandLine = mergeKernelCommandLine(configChain,
		func(c *imagecustomizerapi.Config) []string {
			if c.Pxe != nil {
				return c.Pxe.KernelCommandLine.ExtraCommandLine
			}
			return nil
		})

	// Resolve InitramfsType (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Pxe != nil &&
			configWithBase.Config.Pxe.InitramfsType != "" {
			pxe.InitramfsType = configWithBase.Config.Pxe.InitramfsType
			break
		}
	}

	// Resolve KdumpBootFiles (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Pxe != nil &&
			configWithBase.Config.Pxe.KdumpBootFiles != nil {
			pxe.KdumpBootFiles = configWithBase.Config.Pxe.KdumpBootFiles
			break
		}
	}

	// Resolve BootstrapBaseUrl (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Pxe != nil &&
			configWithBase.Config.Pxe.BootstrapBaseUrl != "" {
			pxe.BootstrapBaseUrl = configWithBase.Config.Pxe.BootstrapBaseUrl
			break
		}
	}

	// Resolve BootstrapFileUrl (current overrides base)
	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Pxe != nil &&
			configWithBase.Config.Pxe.BootstrapFileUrl != "" {
			pxe.BootstrapFileUrl = configWithBase.Config.Pxe.BootstrapFileUrl
			break
		}
	}

	return pxe
}

func resolveCosiCompressionLevel(configChain []*ConfigWithBasePath, cliLevel *int,
	format imagecustomizerapi.ImageFormatType,
) int {
	if cliLevel != nil {
		return *cliLevel
	}

	for _, configWithBase := range slices.Backward(configChain) {
		if configWithBase.Config.Output.Image.Cosi.Compression.Level != nil {
			return *configWithBase.Config.Output.Image.Cosi.Compression.Level
		}
	}

	return defaultCosiCompressionLevel(format)
}

func defaultCosiCompressionLong(format imagecustomizerapi.ImageFormatType) int {
	if format == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		return imagecustomizerapi.DefaultBareMetalCosiCompressionLong
	}
	return imagecustomizerapi.DefaultCosiCompressionLong
}

func defaultCosiCompressionLevel(format imagecustomizerapi.ImageFormatType) int {
	if format == imagecustomizerapi.ImageFormatTypeBareMetalImage {
		return imagecustomizerapi.DefaultBareMetalCosiCompressionLevel
	}
	return imagecustomizerapi.DefaultCosiCompressionLevel
}
