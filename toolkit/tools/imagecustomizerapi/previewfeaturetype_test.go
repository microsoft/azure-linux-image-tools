// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewFeatureIsValid_ValidFeature_Pass(t *testing.T) {
	err := PreviewFeatureUki.IsValid()
	assert.NoError(t, err)
}

func TestPreviewFeatureIsValid_InvalidFeature_Fail(t *testing.T) {
	invalidFeature := PreviewFeature("invalid-feature")
	err := invalidFeature.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid preview feature: invalid-feature")
}
