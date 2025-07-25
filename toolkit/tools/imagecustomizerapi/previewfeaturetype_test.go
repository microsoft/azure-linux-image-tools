// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewFeatureUki_IsValid(t *testing.T) {
	err := PreviewFeatureUki.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeatureOutputArtifacts_IsValid(t *testing.T) {
	err := PreviewFeatureOutputArtifacts.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeatureInjectFiles_IsValid(t *testing.T) {
	err := PreviewFeatureInjectFiles.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeatureReinitializeVerity_IsValid(t *testing.T) {
	err := PreviewFeatureReinitializeVerity.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeaturePackageSnapshotTime_IsValid(t *testing.T) {
	err := PreviewFeaturePackageSnapshotTime.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeatureKdumpBootFiles_IsValid(t *testing.T) {
	err := PreviewFeatureKdumpBootFiles.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeature_IsValid_InvalidValue(t *testing.T) {
	invalidFeature := PreviewFeature("invalid-feature")
	err := invalidFeature.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid preview feature: invalid-feature")
}

func TestPreviewFeature_IsValid_EmptyValue(t *testing.T) {
	emptyFeature := PreviewFeature("")
	err := emptyFeature.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid preview feature:")
}
