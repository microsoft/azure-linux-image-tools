// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewFeature_IsValid(t *testing.T) {
	validFeatures := []PreviewFeature{
		PreviewFeatureUki,
		PreviewFeatureOutputArtifacts,
		PreviewFeatureInjectFiles,
		PreviewFeaturePackageSnapshotTime,
	}

	for _, feature := range validFeatures {
		err := feature.IsValid()
		assert.NoError(t, err, "expected no error for valid feature: %s", feature)
	}

	invalidFeature := PreviewFeature("invalid-feature")
	err := invalidFeature.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid-feature")

	// Test PreviewFeatureReinitializeVerity is not valid (since it's not typed as PreviewFeature)
	var reinitFeature PreviewFeature = PreviewFeatureReinitializeVerity
	err = reinitFeature.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reinitialize-verity")
}
