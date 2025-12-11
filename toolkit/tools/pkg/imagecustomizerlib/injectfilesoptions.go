// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

type InjectFilesOptions struct {
	BuildDir             string
	InputImageFile       string
	OutputImageFile      string
	OutputImageFormat    string
	CosiCompressionLevel int
}

func (o *InjectFilesOptions) IsValid() error {
	if o.CosiCompressionLevel != 0 &&
		(o.CosiCompressionLevel < imagecustomizerapi.MinCosiCompressionLevel ||
			o.CosiCompressionLevel > imagecustomizerapi.MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, o.CosiCompressionLevel,
			imagecustomizerapi.MinCosiCompressionLevel, imagecustomizerapi.MaxCosiCompressionLevel)
	}

	return nil
}

func (o *InjectFilesOptions) verifyPreviewFeatures(previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if o.CosiCompressionLevel != 0 {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
			return ErrCosiCompressionPreviewRequired
		}
	}

	return nil
}
