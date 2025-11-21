// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

func signArtifactsEphemeral(ctx context.Context, rc *signArtifactsResolvedConfig) error {
	// Generate keys.
	certsDir, certificatePath, privateKeyPath, err := ephemeralGenerateKeys(ctx, rc)
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral signing keys:\n%w", err)
	}
	defer os.RemoveAll(certsDir)

	// Sign artifacts.
	err = ephemeralSign(ctx, certificatePath, privateKeyPath, rc)
	if err != nil {
		return fmt.Errorf("failed to sign with ephemeral keys:\n%w", err)
	}

	// Cleanup.
	err = os.RemoveAll(certsDir)
	if err != nil {
		return fmt.Errorf("failed to delete ephemeral private keys:\n%w", err)
	}

	return nil
}

func ephemeralGenerateKeys(ctx context.Context, rc *signArtifactsResolvedConfig) (string, string, string, error) {
	certsDir := filepath.Join(rc.BuildDir, "ephemeral-certs")

	err := os.MkdirAll(certsDir, os.ModePerm)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create ephemeral certificates directory (path='%s):\n%w", certsDir, err)
	}

	certificatePath := filepath.Join(certsDir, "ca.pem")
	privateKeyPath := filepath.Join(certsDir, "ca.private.key")

	err = shell.NewExecBuilder("openssl", "req", "-x509",
		"-newkey", "rsa:2048",
		"-days", "1",
		"-noenc",
		"-keyout", privateKeyPath,
		"-out", certificatePath,
		"-subj", "/CN=Image Customizer Ephemeral Signing",
		"-sha256",
		"-addext", "basicConstraints=CA:FALSE",
		"-addext", "extendedKeyUsage=codeSigning").
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create ephemeral signing certificate:\n%w", err)
	}

	return certsDir, certificatePath, privateKeyPath, nil
}

func ephemeralSign(ctx context.Context, certificatePath string, privateKeyPath string, rc *signArtifactsResolvedConfig,
) error {
	stagingDir := filepath.Join(rc.BuildDir, "ephemeral-staging")
	err := os.MkdirAll(stagingDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral signing staging directory:\n%w", err)
	}

	injectConfig, err := readSignArtifactsMetadataFile(rc.ArtifactsPath)
	if err != nil {
		return err
	}

	for _, item := range injectConfig.InjectFiles {
		itemPath := file.GetAbsPathWithBase(rc.BaseConfigPath, item.Source)

		switch item.Type {
		case imagecustomizerapi.OutputArtifactsItemUkis, imagecustomizerapi.OutputArtifactsItemShim,
			imagecustomizerapi.OutputArtifactsItemSystemdBoot:

			err := ephemeralSignEfi(ctx, itemPath, stagingDir, certificatePath, privateKeyPath)
			if err != nil {
				return fmt.Errorf("failed to sign EFI file (path='%s'):\n%w", itemPath, err)
			}

		case imagecustomizerapi.OutputArtifactsItemVerityHash:
		}
	}

	return nil
}

func ephemeralSignEfi(ctx context.Context, efiPath string, stagingDir string, certificatePath string,
	privateKeyPath string,
) error {
	metadataPath := filepath.Join(stagingDir, "sattrs.bin")

	err := extractEfiSigningAttributesFile(ctx, efiPath, metadataPath)
	if err != nil {
		return err
	}

	return nil
}
