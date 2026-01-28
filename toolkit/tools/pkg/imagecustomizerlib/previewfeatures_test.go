// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestMergePreviewFeatures_BothEmpty_Pass(t *testing.T) {
	result := MergePreviewFeatures(nil, nil)
	assert.Empty(t, result)
}

func TestMergePreviewFeatures_ConfigOnlyEmpty_Pass(t *testing.T) {
	cliFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
	}

	result := MergePreviewFeatures(nil, cliFeatures)
	assert.Equal(t, cliFeatures, result)
}

func TestMergePreviewFeatures_CliOnlyEmpty_Pass(t *testing.T) {
	configFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
	}

	result := MergePreviewFeatures(configFeatures, nil)
	assert.Equal(t, configFeatures, result)
}

func TestMergePreviewFeatures_NoDuplicates_Pass(t *testing.T) {
	configFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
	}
	cliFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}

	result := MergePreviewFeatures(configFeatures, cliFeatures)

	expected := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}
	assert.Equal(t, expected, result)
}

func TestMergePreviewFeatures_WithDuplicates_Pass(t *testing.T) {
	configFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}
	cliFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureOutputArtifacts,
	}

	result := MergePreviewFeatures(configFeatures, cliFeatures)

	expected := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
		imagecustomizerapi.PreviewFeatureOutputArtifacts,
	}
	assert.Equal(t, expected, result)
}

func TestMergePreviewFeatures_AllDuplicates_Pass(t *testing.T) {
	configFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}
	cliFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}

	result := MergePreviewFeatures(configFeatures, cliFeatures)
	assert.Equal(t, configFeatures, result)
}

func TestMergePreviewFeatures_CliDuplicates_Pass(t *testing.T) {
	configFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
	}
	cliFeatures := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureValidateConfig,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}

	result := MergePreviewFeatures(configFeatures, cliFeatures)

	expected := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureValidateConfig,
	}
	assert.Equal(t, expected, result)
}

func TestStringsToPreviewFeatures_Empty_Pass(t *testing.T) {
	result := StringsToPreviewFeatures(nil)
	assert.Nil(t, result)
}

func TestStringsToPreviewFeatures_EmptySlice_Pass(t *testing.T) {
	result := StringsToPreviewFeatures([]string{})
	assert.Nil(t, result)
}

func TestStringsToPreviewFeatures_SingleFeature_Pass(t *testing.T) {
	result := StringsToPreviewFeatures([]string{"uki"})

	expected := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
	}
	assert.Equal(t, expected, result)
}

func TestStringsToPreviewFeatures_MultipleFeatures_Pass(t *testing.T) {
	result := StringsToPreviewFeatures([]string{"uki", "base-configs", "output-artifacts"})

	expected := []imagecustomizerapi.PreviewFeature{
		imagecustomizerapi.PreviewFeatureUki,
		imagecustomizerapi.PreviewFeatureBaseConfigs,
		imagecustomizerapi.PreviewFeatureOutputArtifacts,
	}
	assert.Equal(t, expected, result)
}
