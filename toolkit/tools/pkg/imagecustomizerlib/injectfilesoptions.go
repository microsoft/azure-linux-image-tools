// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import "github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"

type InjectFilesOptions struct {
	BuildDir             string
	InputImageFile       string
	OutputImageFile      string
	OutputImageFormat    string
	CosiCompressionLevel *int
}

func (o *InjectFilesOptions) IsValid() error {
	err := imagecustomizerapi.ValidateCosiCompressionLevel(o.CosiCompressionLevel)
	if err != nil {
		return err
	}

	return nil
}
