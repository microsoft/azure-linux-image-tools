// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

type InjectFilesOptions struct {
	BuildDir             string
	InputImageFile       string
	OutputImageFile      string
	OutputImageFormat    string
	CosiCompressionLevel *int
	TargetOs             targetos.TargetOs
}

func (o *InjectFilesOptions) IsValid() error {
	if o.CosiCompressionLevel != nil &&
		(*o.CosiCompressionLevel < imagecustomizerapi.MinCosiCompressionLevel ||
			*o.CosiCompressionLevel > imagecustomizerapi.MaxCosiCompressionLevel) {
		return fmt.Errorf("%w (level=%d, valid range: %d-%d)",
			ErrInvalidCosiCompressionLevelArg, *o.CosiCompressionLevel,
			imagecustomizerapi.MinCosiCompressionLevel, imagecustomizerapi.MaxCosiCompressionLevel)
	}

	return nil
}

func (o *InjectFilesOptions) verifyPreviewFeatures(previewFeatures []imagecustomizerapi.PreviewFeature) error {
	if o.CosiCompressionLevel != nil {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureCosiCompression) {
			return ErrCosiCompressionPreviewRequired
		}
	}

	if o.TargetOs == targetos.TargetOsUbuntu2204 {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureUbuntu2204) {
			return ErrUbuntu2204PreviewFeatureRequired
		}
	}

	if o.TargetOs == targetos.TargetOsUbuntu2404 {
		if !slices.Contains(previewFeatures, imagecustomizerapi.PreviewFeatureUbuntu2404) {
			return ErrUbuntu2404PreviewFeatureRequired
		}
	}

	return nil
}
