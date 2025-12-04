// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
)

type signArtifactsResolvedConfig struct {
	BuildDir                string
	ArtifactsPath           string
	EphemeralPublicKeysPath string
	BaseConfigPath          string
	Config                  *imagecustomizerapi.SignArtifactsConfig
	InjectFilesConfig       imagecustomizerapi.InjectFilesConfig
}

func resolveSignArtifactsConfig(baseConfigPath string, config *imagecustomizerapi.SignArtifactsConfig,
	options SignArtifactsOptions,
) (*signArtifactsResolvedConfig, error) {
	var err error

	rc := signArtifactsResolvedConfig{
		Config: config,
	}

	err = config.IsValid()
	if err != nil {
		return nil, fmt.Errorf("config is invalid:\n%w", err)
	}

	rc.BuildDir, err = os.MkdirTemp(options.BuildDir, "sign-artifacts-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary build directory:\n%w", err)
	}

	rc.BaseConfigPath = baseConfigPath

	rc.ArtifactsPath, err = resolveSignArtifactsPath(options.ArtifactsPath, rc.BaseConfigPath, config)
	if err != nil {
		return nil, err
	}

	rc.EphemeralPublicKeysPath, err = resolveEphemeralPublicKeysPath(options.EphemeralPublicKeysPath, rc.BaseConfigPath,
		config)
	if err != nil {
		return nil, err
	}

	rc.InjectFilesConfig, err = readInjectFilesMetadataFile(rc.ArtifactsPath)
	if err != nil {
		return nil, err
	}

	return &rc, nil
}

func resolveSignArtifactsPath(cliArtifactsPath string, baseConfigPath string,
	config *imagecustomizerapi.SignArtifactsConfig,
) (string, error) {
	if cliArtifactsPath != "" {
		return cliArtifactsPath, nil
	}

	if config.Input.ArtifactsPath != "" {
		artifactsPath := file.GetAbsPathWithBase(baseConfigPath, config.Input.ArtifactsPath)
		return artifactsPath, nil
	}

	return "", fmt.Errorf("no artifacts path provided")
}

func resolveEphemeralPublicKeysPath(cliEphemeralPublicKeysPath string, baseConfigPath string,
	config *imagecustomizerapi.SignArtifactsConfig,
) (string, error) {
	if cliEphemeralPublicKeysPath != "" {
		if config.SigningMethod.Ephemeral == nil {
			return "", fmt.Errorf("cannot specify --ephemeral-public-keys-path if ephemeral signing method is not used")
		}

		return cliEphemeralPublicKeysPath, nil
	}

	if config.SigningMethod.Ephemeral != nil {
		if config.SigningMethod.Ephemeral.PublicKeysPath != "" {
			publicKeysPath := file.GetAbsPathWithBase(baseConfigPath, config.SigningMethod.Ephemeral.PublicKeysPath)
			return publicKeysPath, nil
		}

		return "", fmt.Errorf("no public keys path provided for ephemeral keys signing method")
	}

	return "", nil
}

func readInjectFilesMetadataFile(artifactsPath string) (imagecustomizerapi.InjectFilesConfig, error) {
	injectFilesConfigPath := filepath.Join(artifactsPath, InjectFilesName)

	var injectConfig imagecustomizerapi.InjectFilesConfig
	err := imagecustomizerapi.UnmarshalYamlFile(injectFilesConfigPath, &injectConfig)
	if err != nil {
		err = fmt.Errorf("failed to read %s file:\n%w", InjectFilesName, err)
		return imagecustomizerapi.InjectFilesConfig{}, err
	}

	if err := injectConfig.IsValid(); err != nil {
		err = fmt.Errorf("invalid %s file:\n%w", InjectFilesName, err)
		return imagecustomizerapi.InjectFilesConfig{}, err
	}

	return injectConfig, nil
}
