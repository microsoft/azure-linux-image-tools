// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"slices"

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
	// Note: BuildDir is validated in ConvertImageWithOptions() because it's only required
	// for COSI/bare-metal-image output formats, not for simple format conversions.

	if o.InputImageFile == "" {
		return fmt.Errorf("input image file must be specified")
	}

	if o.OutputImageFile == "" {
		return fmt.Errorf("output image file must be specified")
	}

	if o.OutputImageFormat == "" {
		return fmt.Errorf("output image format must be specified")
	}

	if err := validateCosiCompressionLevel(o.CosiCompressionLevel); err != nil {
		return err
	}

	return nil
}

func (o *ConvertImageOptions) verifyPreviewFeatures(previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureConvert) {
		return ErrConvertPreviewRequired
	}

	if err := verifyCosiCompressionPreviewFeature(o.CosiCompressionLevel, previewFeatures); err != nil {
		return err
	}

	return nil
}

// validateCosiCompressionLevel validates the COSI compression level value.
func validateCosiCompressionLevel(level *int) error {
	if level != nil &&
		(*level < imagecustomizerapi.MinCosiCompressionLevel ||
			*level > imagecustomizerapi.MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *level,
			imagecustomizerapi.MinCosiCompressionLevel, imagecustomizerapi.MaxCosiCompressionLevel)
	}
	return nil
}

// verifyCosiCompressionPreviewFeature verifies the COSI compression preview feature is enabled when needed.
func verifyCosiCompressionPreviewFeature(level *int, previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if level != nil {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
			return ErrCosiCompressionPreviewRequired
		}
	}
	return nil
}
