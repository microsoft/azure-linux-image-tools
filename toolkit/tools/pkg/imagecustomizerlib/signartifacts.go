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
		return fmt.Errorf("failed to extract EFI file's signing metadata (path='%s'):\n%w", efiPath, err)
	}

	return nil
}

func attachEfiSignature(ctx context.Context, efiPath string, attributesPath string, signaturePath string, certificatePath string,
	stagingDir string,
) error {
	// pesign uses NSS to manage certificates and private keys.
	// Unfortunately, pesign doesn't provide an option to provide the public certificate as a normal CLI arg. So, we
	// have to pass it via an NSS database instead. :-(
	nssCertName := "newcert"
	nssDbPath := filepath.Join(stagingDir, "nssdb")

	err := os.Mkdir(nssDbPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create NSS database directory (path='%s'):\n%w", nssDbPath, err)
	}
	defer os.RemoveAll(nssDbPath)

	// Initialize NSS database.
	err = shell.NewExecBuilder("certutil", "-N", "-d", nssDbPath, "--empty-password").
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to initialize NSS database (path='%s'):\n%w", nssDbPath, err)
	}

	// Import certificate into NSS database.
	err = shell.NewExecBuilder("certutil", "-A", "-n", nssCertName, "-d", nssDbPath, "-i", certificatePath, "-t", ",,u").
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to import certificate into NSS database (path='%s'):\n%w", certificatePath, err)
	}

	// Attach signature to EFI file.
	efiTmpPath := efiPath + ".tmp"

	err = shell.NewExecBuilder(
		"pesign",
		"--certificate", nssCertName,
		"--import-raw-signature", signaturePath,
		"--import-signed-attributes", attributesPath,
		"--in", efiPath,
		"--out", efiTmpPath,
		"--certdir", nssDbPath).
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to attached signature to EFI file (sig='%s', efi='%s'):\n%w", signaturePath, efiPath,
			err)
	}

	err = os.Rename(efiTmpPath, efiPath)
	if err != nil {
		return fmt.Errorf("failed to rename temporary EFI file (from='%s', to='%s'):\n%w", efiTmpPath, efiPath, err)
	}

	return nil
}
