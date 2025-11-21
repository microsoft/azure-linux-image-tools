// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type signArtifactsResolvedConfig struct {
	BuildDir                string
	ArtifactsPath           string
	EphemeralPublicKeysPath string
	BaseConfigPath          string
	Config                  *imagecustomizerapi.SignArtifactsConfig
}

func resolveSignArtifactsConfig(baseConfigPath string, config *imagecustomizerapi.SignArtifactsConfig,
	options SignArtifactsOptions,
) (*signArtifactsResolvedConfig, error) {
	var err error

	rc := signArtifactsResolvedConfig{
		Config: config,
	}

	buildDirAbs, err := filepath.Abs(options.BuildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of build directory:\n%w", err)
	}

	rc.BuildDir, err = os.MkdirTemp(buildDirAbs, "sign-artifacts-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary build directory:\n%w", err)
	}

	rc.BaseConfigPath, err = filepath.Abs(baseConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrGetAbsoluteConfigPath, err)
	}

	rc.ArtifactsPath, err = resolveSignArtifactsPath(options.ArtifactsPath)
	if err != nil {
		return nil, err
	}

	rc.EphemeralPublicKeysPath, err = resolveEphemeralPublicKeysPath(options.EphemeralPublicKeysPath)
	if err != nil {
		return nil, err
	}

	return &rc, nil
}

func resolveSignArtifactsPath(cliArtifactsPath string) (string, error) {
	if cliArtifactsPath != "" {
		fileInfo, err := os.Stat(cliArtifactsPath)
		if err != nil {
			return "", fmt.Errorf("could not find artifacts path (path='%s'):\n%w", cliArtifactsPath, err)
		}

		if !fileInfo.IsDir() {
			return "", fmt.Errorf("artifacts path is not a directory (path='%s'):\n%w", cliArtifactsPath, err)
		}

		return cliArtifactsPath, nil
	}

	return "", fmt.Errorf("no artifacts path provided")
}

func resolveEphemeralPublicKeysPath(cliEphemeralPublicKeysPath string) (string, error) {
	if cliEphemeralPublicKeysPath != "" {
		return cliEphemeralPublicKeysPath, nil
	}

	return "", fmt.Errorf("no public keys path provided for ephemeral keys signing method")
}
