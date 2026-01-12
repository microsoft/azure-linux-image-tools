// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewFeatureIsValid_ValidFeatures_Pass(t *testing.T) {
	validFeatures := []PreviewFeature{
		PreviewFeatureUki,
		PreviewFeatureOutputArtifacts,
		PreviewFeatureInjectFiles,
		PreviewFeatureReinitializeVerity,
		PreviewFeaturePackageSnapshotTime,
		PreviewFeatureKdumpBootFiles,
		PreviewFeatureFedora42,
		PreviewFeatureBaseConfigs,
		PreviewFeatureInputImageOci,
		PreviewFeatureOutputSelinuxPolicy,
		PreviewFeatureCosiCompression,
		PreviewFeatureBtrfs,
	}

	for _, feature := range validFeatures {
		t.Run(string(feature), func(t *testing.T) {
			err := feature.IsValid()
			assert.NoError(t, err)
		})
	}
}

func TestPreviewFeatureIsValid_InvalidFeature_Fail(t *testing.T) {
	invalidFeature := PreviewFeature("invalid-feature")
	err := invalidFeature.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid preview feature: invalid-feature")
}
