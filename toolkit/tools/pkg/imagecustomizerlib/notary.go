// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/dir"
	"github.com/notaryproject/notation-go/registry"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/notaryproject/notation-go/verifier/truststore"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

type ociSignatureCheckOptions struct {
	TrustPolicyName   string
	TrustStoreName    string
	CertificateFs     fs.FS
	CertificateFsPath string
}

func checkNotationSignature(ctx context.Context, buildDir string, remoteRepo *remote.Repository,
	descriptor ociv1.Descriptor, options ociSignatureCheckOptions,
) error {
	// Recreate the full artifact reference URI for the provided digest.
	reference := remoteRepo.Reference
	reference.Reference = descriptor.Digest.String()
	artifactReference := reference.String()

	logger.Log.Debugf("Verifying OCI signature (%s)", artifactReference)

	trustPolicy := &trustpolicy.Document{
		Version: "1.0",
		TrustPolicies: []trustpolicy.TrustPolicy{
			{
				Name: options.TrustPolicyName,
				RegistryScopes: []string{
					"*",
				},
				SignatureVerification: trustpolicy.SignatureVerification{
					VerificationLevel: "strict",
				},
				TrustStores: []string{
					string(truststore.TypeCA) + ":" + options.TrustStoreName,
				},
				TrustedIdentities: []string{
					"*",
				},
			},
		},
	}

	trustStorePath, err := os.MkdirTemp(buildDir, "trust-store-path-")
	if err != nil {
		return fmt.Errorf("failed to create OCI signature check certificate store:\n%w", err)
	}
	defer os.RemoveAll(trustStorePath)

	// Create a directory to use as a trust store.
	trustStoreFs := dir.NewSysFS(trustStorePath)
	certDestinationDir, err := trustStoreFs.SysPath(dir.TrustStoreDir, "x509", string(truststore.TypeCA),
		options.TrustStoreName)
	if err != nil {
		return err
	}

	certDestinationPath := filepath.Join(certDestinationDir,
		filepath.Base(options.CertificateFsPath))

	// Copy certificate into trust store.
	err = file.CopyResourceFile(options.CertificateFs, options.CertificateFsPath,
		certDestinationPath, os.ModePerm, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create OCI signature check certificate file (%s):\n%w", certDestinationPath, err)
	}

	// Create trust store.
	trustStore := truststore.NewX509TrustStore(trustStoreFs)

	// Create verifier.
	verifierOptions := verifier.VerifierOptions{}

	verifier, err := verifier.NewWithOptions(trustPolicy, trustStore, nil, verifierOptions)
	if err != nil {
		return err
	}

	verifyOptions := notation.VerifyOptions{
		ArtifactReference:    artifactReference,
		MaxSignatureAttempts: 50, // Max number of signatures attached to artifact.
	}

	notaryRepo := registry.NewRepository(remoteRepo)

	// Verify signature.
	_, _, err = notation.Verify(ctx, verifier, notaryRepo, verifyOptions)
	if err != nil {
		return err
	}

	return nil
}
