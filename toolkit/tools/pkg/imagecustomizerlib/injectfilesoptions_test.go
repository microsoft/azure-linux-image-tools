// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestInjectFilesOptionsIsValid_ValidOptions_Pass(t *testing.T) {
	options := InjectFilesOptions{}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestInjectFilesOptionsIsValid_CompressionLevel_Pass(t *testing.T) {
	testCases := []int{1, 9, 15, 22}
	for _, level := range testCases {
		options := InjectFilesOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.NoError(t, err, "level %d should be valid", level)
	}
}

func TestInjectFilesOptionsIsValid_CompressionLevel_Fail(t *testing.T) {
	testCases := []int{-1, 0, 23, 100}
	for _, level := range testCases {
		options := InjectFilesOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.ErrorIs(t, err, ErrInvalidCosiCompressionLevelArg, "level %d should be invalid", level)
	}
}

func TestInjectFilesOptionsVerifyPreviewFeatures_CosiCompressionLevelNoFeature_Fail(t *testing.T) {
	level := 15
	options := InjectFilesOptions{
		CosiCompressionLevel: &level,
	}
	previewFeatures := []imagecustomizerapi.PreviewFeature{}
	err := options.verifyPreviewFeatures(previewFeatures)
	assert.ErrorIs(t, err, ErrCosiCompressionPreviewRequired)
}

func TestInjectFilesOptionsVerifyPreviewFeatures_CosiCompressionLevelWithFeature_Pass(t *testing.T) {
	level := 15
	options := InjectFilesOptions{
		CosiCompressionLevel: &level,
	}
	previewFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureCosiCompression,
	}
	err := options.verifyPreviewFeatures(previewFeatures)
	assert.NoError(t, err)
}

func TestInjectFilesOptionsVerifyPreviewFeatures_NoCosiCompressionLevelWithFeature_Pass(t *testing.T) {
	options := InjectFilesOptions{}
	previewFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureCosiCompression,
	}
	err := options.verifyPreviewFeatures(previewFeatures)
	assert.NoError(t, err)
}

func TestInjectFilesOptionsVerifyPreviewFeatures_NoCosiCompressionLevelNoFeature_Pass(t *testing.T) {
	options := InjectFilesOptions{}
	previewFeatures := []imagecustomizerapi.PreviewFeature{}
	err := options.verifyPreviewFeatures(previewFeatures)
	assert.NoError(t, err)
}
