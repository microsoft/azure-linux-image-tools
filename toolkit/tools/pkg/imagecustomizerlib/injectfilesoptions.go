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
	CosiCompressionLevel *int
}

func (o *InjectFilesOptions) IsValid() error {
	if o.CosiCompressionLevel != nil &&
		(*o.CosiCompressionLevel < imagecustomizerapi.MinZstdCompressionLevel ||
			*o.CosiCompressionLevel > imagecustomizerapi.MaxZstdCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *o.CosiCompressionLevel,
			imagecustomizerapi.MinZstdCompressionLevel, imagecustomizerapi.MaxZstdCompressionLevel)
	}

	return nil
}

func (o *InjectFilesOptions) verifyPreviewFeatures(previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if o.CosiCompressionLevel != nil {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
			return ErrCosiCompressionPreviewRequired
		}
	}

	return nil
}
