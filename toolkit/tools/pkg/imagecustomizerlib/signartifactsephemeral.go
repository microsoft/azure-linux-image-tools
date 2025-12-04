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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

func signArtifactsEphemeral(ctx context.Context, rc *signArtifactsResolvedConfig) error {
	logger.Log.Infof("Signing artifacts with ephemeral keys")

	// Create public keys output directory.
	err := os.MkdirAll(rc.EphemeralPublicKeysPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral public keys output directory (path='%s'):\n%w",
			rc.EphemeralPublicKeysPath, err)
	}

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

	// Output public keys.
	logger.Log.Infof("Outputting public keys")
	outputCertificatePath := filepath.Join(rc.EphemeralPublicKeysPath, "ca.pem")
	err = file.Copy(certificatePath, outputCertificatePath)
	if err != nil {
		return fmt.Errorf("failed to output public certificate (path='%s'):\n%w", err)
	}

	// Cleanup.
	err = os.RemoveAll(certsDir)
	if err != nil {
		return fmt.Errorf("failed to delete ephemeral private keys:\n%w", err)
	}

	logger.Log.Infof("Success!")

	return nil
}

func ephemeralGenerateKeys(ctx context.Context, rc *signArtifactsResolvedConfig) (string, string, string, error) {
	logger.Log.Infof("Generating ephemeral signing keys")

	certsDir := filepath.Join(rc.BuildDir, "ephemeral-certs")

	err := os.MkdirAll(certsDir, os.ModePerm)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create ephemeral certificates directory (path='%s):\n%w", certsDir, err)
	}

	certificatePath := filepath.Join(certsDir, "ca.pem")
	privateKeyPath := filepath.Join(certsDir, "ca.private.key")

	// For UEFI Secure Boot, the certificate common name (CN) is only used as a friendly name.
	// While it doesn't need to be unique, making it unique can make life easier for the user.
	commonNameSuffix, _ := randomization.RandomString(4, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	commonName := "Image Customizer Ephemeral Signing " + commonNameSuffix

	err = shell.NewExecBuilder("openssl", "req", "-x509",
		"-newkey", "rsa:2048",
		"-days", "1",
		"-noenc",
		"-keyout", privateKeyPath,
		"-out", certificatePath,
		"-subj", "/CN="+commonName,
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

	for _, item := range rc.InjectFilesConfig.InjectFiles {
		logger.Log.Infof("Signing artifact: %s", item.Source)

		itemPath := file.GetAbsPathWithBase(rc.ArtifactsPath, item.Source)

		switch item.Type {
		case imagecustomizerapi.OutputArtifactsItemUkis, imagecustomizerapi.OutputArtifactsItemShim,
			imagecustomizerapi.OutputArtifactsItemSystemdBoot:

			err := ephemeralSignEfi(ctx, itemPath, stagingDir, certificatePath, privateKeyPath)
			if err != nil {
				return fmt.Errorf("failed to sign EFI file (path='%s'):\n%w", itemPath, err)
			}

		case imagecustomizerapi.OutputArtifactsItemVerityHash:
			err := ephemeralSignVerityHash(ctx, itemPath, privateKeyPath)
			if err != nil {
				return fmt.Errorf("failed to sign verity hash file (path='%s'):\n%w", itemPath, err)
			}
		}
	}

	return nil
}

func ephemeralSignEfi(ctx context.Context, efiPath string, stagingDir string, certificatePath string,
	privateKeyPath string,
) error {
	metadataPath := filepath.Join(stagingDir, "sattrs.bin")
	signaturePath := filepath.Join(stagingDir, "sattrs.bin.sig")

	err := extractEfiSigningAttributesFile(ctx, efiPath, metadataPath)
	if err != nil {
		return err
	}

	err = createSignatureOfFile(ctx, metadataPath, signaturePath, privateKeyPath)
	if err != nil {
		return err
	}

	err = attachEfiSignature(ctx, efiPath, metadataPath, signaturePath, certificatePath, stagingDir)
	if err != nil {
		return err
	}

	return nil
}

func ephemeralSignVerityHash(ctx context.Context, verityHashPath string, privateKeyPath string,
) error {
	signaturePath := verityHashPath + ".tmp"

	err := createSignatureOfDigest(ctx, verityHashPath, signaturePath, privateKeyPath)
	if err != nil {
		return err
	}

	err = os.Rename(signaturePath, verityHashPath)
	if err != nil {
		return fmt.Errorf("failed to rename verity signature file (from='%s', to='%s'):\n%w", signaturePath,
			verityHashPath, err)
	}

	return nil
}

// Hash a file and then create a signature from the hash.
func createSignatureOfFile(ctx context.Context, path string, signaturePath string, privateKeyPath string) error {
	err := shell.NewExecBuilder("openssl", "dgst", "-sign", privateKeyPath, "-sha256", "-out", signaturePath, path).
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to create signature for file (path='%s'):\n%w", path, err)
	}

	return nil
}

// Directly sign a digest.
func createSignatureOfDigest(ctx context.Context, path string, signaturePath string, privateKeyPath string) error {
	// TODO: Convert HEX to binary.
	// TODO: Wrong output format?
	err := shell.NewExecBuilder("openssl", "pkeyutl", "-sign",
		"-in", path,
		"-inkey", privateKeyPath,
		"-out", signaturePath,
		// Specify the type of digest being provided.
		"-pkeyopt", "digest:sha256").
		Context(ctx).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to create signature for file (path='%s'):\n%w", path, err)
	}

	return nil
}
