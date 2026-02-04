// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type ConvertImageOptions struct {
	BuildDir             string
	InputImageFile       string
	OutputImageFile      string
	OutputImageFormat    imagecustomizerapi.ImageFormatType
	CosiCompressionLevel *int
}

func (o *ConvertImageOptions) IsValid() error {
	if o.InputImageFile == "" {
		return ErrInputImageFileRequired
	}

	if o.OutputImageFile == "" {
		return ErrOutputImageFileRequired
	}

	if err := o.OutputImageFormat.IsValid(); err != nil {
		return fmt.Errorf("%w (format='%s'):\n%w", ErrInvalidOutputFormat, o.OutputImageFormat, err)
	}

	outputFormat := o.OutputImageFormat

	// Build directory is required for COSI and bare-metal-image output formats
	requiresBuildDir := outputFormat == imagecustomizerapi.ImageFormatTypeCosi ||
		outputFormat == imagecustomizerapi.ImageFormatTypeBareMetalImage
	if requiresBuildDir && o.BuildDir == "" {
		return ErrConvertBuildDirRequired
	}

	// COSI compression level can only be specified for COSI or bare-metal-image output formats
	if o.CosiCompressionLevel != nil {
		if !requiresBuildDir {
			return fmt.Errorf("COSI compression level can only be specified for COSI or bare-metal-image output formats")
		}
	}

	if err := imagecustomizerapi.ValidateCosiCompressionLevel(o.CosiCompressionLevel); err != nil {
		return err
	}

	return nil
}
