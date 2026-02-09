// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// downloadAzureLinuxImage downloads an Azure Linux image from MCR.
// buildDir must exist and be a writable directory when a descriptor is not provided.
func downloadAzureLinuxImage(ctx context.Context, inputImage imagecustomizerapi.AzureLinuxImage,
	ociDescriptor *ociv1.Descriptor, buildDir string, imageCacheDir string,
) (string, error) {
	ociImage, err := generateAzureLinuxOciUri(inputImage)
	if err != nil {
		return "", err
	}

	inputImageFilePath, err := downloadOciImage(ctx, ociImage, ociDescriptor, buildDir, imageCacheDir,
		getAzureLinuxOciSignatureCheckOptions())
	if err != nil {
		return "", err
	}

	return inputImageFilePath, nil
}

func generateAzureLinuxOciUri(inputImage imagecustomizerapi.AzureLinuxImage) (imagecustomizerapi.OciImage, error) {
	majorMinor, date, err := inputImage.ParseVersion()
	if err != nil {
		return imagecustomizerapi.OciImage{}, err
	}

	tag := "latest"
	if date != "" {
		tag = majorMinor + "." + date
	}

	// Note: 'majorMinor', 'tag' and 'variant' are already sanitized.
	// So, there is no need to escape the values.
	uri := fmt.Sprintf("mcr.microsoft.com/azurelinux/%s/image/%s:%s", majorMinor, inputImage.Variant, tag)
	ociImage := imagecustomizerapi.OciImage{
		Uri: uri,
	}

	return ociImage, nil
}

func parseInputImageAzureLinuxValue(value string) (imagecustomizerapi.InputImage, error) {
	variant, version, splitOk := strings.Cut(value, ":")
	if !splitOk {
		err := fmt.Errorf("missing Azure Linux version value")
		return imagecustomizerapi.InputImage{}, err
	}

	inputImage := imagecustomizerapi.InputImage{
		AzureLinux: &imagecustomizerapi.AzureLinuxImage{
			Variant: variant,
			Version: version,
		},
	}

	err := inputImage.AzureLinux.IsValid()
	if err != nil {
		return imagecustomizerapi.InputImage{}, err
	}

	return inputImage, nil
}

func getAzureLinuxOciSignatureCheckOptions() *ociSignatureCheckOptions {
	return &ociSignatureCheckOptions{
		TrustPolicyName:   "mcr-azure-linux",
		TrustStoreName:    "microsoft-supplychain",
		CertificateFs:     resources.ResourcesFS,
		CertificateFsPath: resources.MicrosoftSupplyChainRSARootCA2022File,
	}
}
