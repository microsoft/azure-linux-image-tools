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
	OutputImageFormat    string
	CosiCompressionLevel *int
}

func (o *ConvertImageOptions) IsValid() error {
	if o.BuildDir == "" {
		return fmt.Errorf("build directory must be specified")
	}

	if o.InputImageFile == "" {
		return fmt.Errorf("input image file must be specified")
	}

	if o.OutputImageFile == "" {
		return fmt.Errorf("output image file must be specified")
	}

	if o.OutputImageFormat == "" {
		return fmt.Errorf("output image format must be specified")
	}

	if o.CosiCompressionLevel != nil &&
		(*o.CosiCompressionLevel < imagecustomizerapi.MinCosiCompressionLevel ||
			*o.CosiCompressionLevel > imagecustomizerapi.MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *o.CosiCompressionLevel,
			imagecustomizerapi.MinCosiCompressionLevel, imagecustomizerapi.MaxCosiCompressionLevel)
	}

	return nil
}
