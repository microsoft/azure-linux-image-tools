// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

var (
	ErrDownloadImageOci        = NewImageCustomizerError("DownloadImage:Oci", "failed to download image from OCI artifact")
	ErrDownloadImageAzureLinux = NewImageCustomizerError("DownloadImage:AzureLinux", "failed to download Azure Linux image")
)

func downloadImage(ctx context.Context, inputImage imagecustomizerapi.InputImage, buildDir string, imageCacheDir string,
) (string, error) {
	switch {
	case inputImage.Path != "":
		return inputImage.Path, nil

	case inputImage.Oci != nil:
		inputImageFilePath, err := downloadOciImage(ctx, *inputImage.Oci, buildDir, imageCacheDir)
		if err != nil {
			return "", fmt.Errorf("%w:\n%w", ErrDownloadImageOci, err)
		}

		return inputImageFilePath, nil

	case inputImage.AzureLinux != nil:
		inputImageFilePath, err := downloadAzureLinuxImage(ctx, *inputImage.AzureLinux, buildDir, imageCacheDir)
		if err != nil {
			return "", fmt.Errorf("%w:\n%w", ErrDownloadImageAzureLinux, err)
		}

		return inputImageFilePath, nil

	default:
		panic("inputImage doesn't contain a value")
	}
}
