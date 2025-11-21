// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

type SignArtifactsOptions struct {
	BuildDir                string
	ArtifactsPath           string
	EphemeralPublicKeysPath string
}

func SignArtifactsWithConfigFile(ctx context.Context, configFile string, options SignArtifactsOptions) error {
	var config imagecustomizerapi.SignArtifactsConfig
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	if err != nil {
		return err
	}

	baseConfigPath, _ := filepath.Split(configFile)

	err = SignArtifacts(ctx, baseConfigPath, &config, options)
	if err != nil {
		return err
	}

	return nil
}

func SignArtifacts(ctx context.Context, baseConfigPath string, config *imagecustomizerapi.SignArtifactsConfig,
	options SignArtifactsOptions,
) error {
	rc, err := resolveSignArtifactsConfig(baseConfigPath, config, options)
	if err != nil {
		return err
	}
	defer signArtifactsCleanup(rc)

	switch {
	case rc.Config.SigningMethod.Ephemeral != nil:
		err := signArtifactsEphemeral(ctx, rc)
		if err != nil {
			return fmt.Errorf("failed to sign artifacts with ephemeral keys:\n%w", err)
		}
	}

	err = signArtifactsCleanup(rc)
	if err != nil {
		return fmt.Errorf("failed to cleanup:\n%w", err)
	}

	return nil
}

func signArtifactsCleanup(rc *signArtifactsResolvedConfig) error {
	errs := []error(nil)

	err := os.RemoveAll(rc.BuildDir)
	if err != nil {
		err = fmt.Errorf("failed to delete temp build directory (path='%s'):\n%w", rc.BuildDir, err)
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func readSignArtifactsMetadataFile(artifactsPath string) (imagecustomizerapi.InjectFilesConfig, error) {
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

// Extract the metadata blob from an EFI file that can be used for signing the EFI file.
// This blob can be signed by any compliant x509 signing service.
// Note: While 'pesign' can handle the full signing operation itself in one go, splitting up the steps makes it easier
// to plug-in custom signing solutions.
func extractEfiSigningAttributesFile(ctx context.Context, efiPath string, attributesPath string) error {
	err := shell.NewExecBuilder("pesign", "--force",
		"--in", efiPath,
		"--export-signed-attributes", attributesPath).
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to extract efi file signing metadata (path='%s'):\n%w", efiPath, err)
	}

	return nil
}
