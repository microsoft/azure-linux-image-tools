// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItemIsValid_ValidItems(t *testing.T) {
	validItems := []OutputArtifactsItemType{
		OutputArtifactsItemUkis,
		OutputArtifactsItemShim,
		OutputArtifactsItemBootloader,
		OutputArtifactsItemVerityHash,
		OutputArtifactsItemDefault,
	}

	for _, item := range validItems {
		err := item.IsValid()
		assert.NoError(t, err, "valid item should not return an error: %s", item)
	}
}

func TestItemIsValid_UkiAddons_Fail(t *testing.T) {
	err := OutputArtifactsItemUkiAddons.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "uki-addons are automatically included with ukis")
}

func TestItemIsValid_InvalidItem(t *testing.T) {
	invalidItem := OutputArtifactsItemType("invalidItem")
	err := invalidItem.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}
