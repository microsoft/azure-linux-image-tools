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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/dir"
	"github.com/notaryproject/notation-go/registry"
	"github.com/notaryproject/notation-go/verifier"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"github.com/notaryproject/notation-go/verifier/truststore"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	orasregistry "oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	// The maximum number of singatures an OCI artifact is expected to have.
	// This is mostly just a denial-of-service protection limit.
	// So, the number is set to be unrealistically large.
	maxOciSignatures = 50
)

type ociSignatureCheckOptions struct {
	TrustPolicyName   string
	TrustStoreName    string
	CertificateFs     fs.FS
	CertificateFsPath string
}

func getAzureLinuxOciSignatureCheckOptions() *ociSignatureCheckOptions {
	return &ociSignatureCheckOptions{
		TrustPolicyName:   "mcr-azure-linux",
		TrustStoreName:    "microsoft-supplychain",
		CertificateFs:     resources.ResourcesFS,
		CertificateFsPath: resources.MicrosoftSupplyChainRSARootCA2022File,
	}
}

func checkNotationSignature(ctx context.Context, buildDir string, remoteRepo *remote.Repository,
	descriptor ociv1.Descriptor, options ociSignatureCheckOptions,
) error {
	// Recreate the full artifact reference URI for the provided digest.
	digestUri := createOciDigestUri(remoteRepo.Reference, descriptor)
	logger.Log.Debugf("Verifying OCI signature (%s)", digestUri)

	trustStorePath, err := os.MkdirTemp(buildDir, "trust-store-path-")
	if err != nil {
		return fmt.Errorf("failed to create OCI signature check certificate store:\n%w", err)
	}
	defer os.RemoveAll(trustStorePath)

	trustStoreType := truststore.TypeCA
	trustStore, err := createNotationX509TrustStore(trustStorePath, options.TrustStoreName, trustStoreType,
		options.CertificateFs, options.CertificateFsPath)
	if err != nil {
		return err
	}

	verifier, err := createNotationX509Verifier(options.TrustPolicyName, options.TrustStoreName, trustStoreType,
		trustStore)
	if err != nil {
		return err
	}

	verifyOptions := notation.VerifyOptions{
		ArtifactReference:    digestUri,
		MaxSignatureAttempts: maxOciSignatures,
	}

	notaryRepo := registry.NewRepository(remoteRepo)

	// Verify signature.
	_, _, err = notation.Verify(ctx, verifier, notaryRepo, verifyOptions)
	if err != nil {
		return err
	}

	return nil
}

// Create the full digest URI of an OCI artifact.
func createOciDigestUri(registry orasregistry.Reference, artifact ociv1.Descriptor) string {
	reference := registry
	reference.Reference = artifact.Digest.String()
	digestUri := reference.String()
	return digestUri
}

// Create a Notation trust store.
// This is mostly just a directory that contains the CA's public certificate. Though Notation has a specific directory
// layout that you have to follow.
func createNotationX509TrustStore(trustStorePath string, trustStoreName string, trustStoreType truststore.Type,
	certificateFs fs.FS, certificateFsPath string,
) (truststore.X509TrustStore, error) {
	// Create a directory to use as a trust store.
	trustStoreFs := dir.NewSysFS(trustStorePath)
	certDestinationDir, err := trustStoreFs.SysPath(dir.X509TrustStoreDir(string(trustStoreType), trustStoreName))
	if err != nil {
		return nil, err
	}

	certDestinationPath := filepath.Join(certDestinationDir, filepath.Base(certificateFsPath))

	// Copy certificate into trust store.
	err = file.CopyResourceFile(certificateFs, certificateFsPath, certDestinationPath, os.ModePerm, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI signature check certificate file (%s):\n%w", certDestinationPath,
			err)
	}

	// Create Notation trust store.
	trustStore := truststore.NewX509TrustStore(trustStoreFs)
	return trustStore, nil
}

// Create a Notation verifier from an x509 trust store.
func createNotationX509Verifier(trustPolicyName string, trustStoreName string, trustStoreType truststore.Type,
	trustStore truststore.X509TrustStore,
) (notation.Verifier, error) {
	trustPolicy := &trustpolicy.Document{
		Version: "1.0",
		TrustPolicies: []trustpolicy.TrustPolicy{
			{
				Name: trustPolicyName,
				RegistryScopes: []string{
					// Appply policy to all OCI artifacts.
					"*",
				},
				SignatureVerification: trustpolicy.SignatureVerification{
					VerificationLevel: "strict",
				},
				TrustStores: []string{
					// The sub trust-stores to use in this policy.
					string(trustStoreType) + ":" + trustStoreName,
				},
				TrustedIdentities: []string{
					// The identities that are permitted in the sub trust-stores.
					"*",
				},
			},
		},
	}

	// Create verifier.
	verifierOptions := verifier.VerifierOptions{}

	verifier, err := verifier.NewWithOptions(trustPolicy, trustStore, nil, verifierOptions)
	if err != nil {
		return nil, err
	}

	return verifier, nil
}
