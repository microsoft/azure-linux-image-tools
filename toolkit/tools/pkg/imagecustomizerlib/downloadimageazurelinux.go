// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

func downloadAzureLinuxImage(ctx context.Context, inputImage imagecustomizerapi.AzureLinuxImage, buildDir string,
	imageCacheDir string,
) (string, error) {
	majorMinor, date, err := inputImage.ParseVersion()
	if err != nil {
		return "", err
	}

	tag := "latest"
	if date != "" {
		tag = majorMinor + "." + date
	}

	// Note: 'majorMinor' and 'tag' are already sanitized. So, there is no need to escape the values.
	uri := fmt.Sprintf("mcr.microsoft.com/azurelinux/%s/image/minimal-os:%s", majorMinor, tag)
	ociImage := imagecustomizerapi.OciImage{
		Uri: uri,
	}

	inputImageFilePath, err := downloadOciImage(ctx, ociImage, buildDir, imageCacheDir)
	if err != nil {
		return "", err
	}

	return inputImageFilePath, err
}
