// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputArtifactsItemTypeIsValid_ValidItems_Pass(t *testing.T) {
	validItems := []OutputArtifactsItemType{
		OutputArtifactsItemUkis,
		OutputArtifactsItemUkiAddons,
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

func TestOutputArtifactsItemTypeIsValid_InvalidItem_Fail(t *testing.T) {
	invalidItem := OutputArtifactsItemType("invalidItem")
	err := invalidItem.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}

func TestOutputArtifactsItemTypeIsValidOutputItem_ValidItems_Pass(t *testing.T) {
	validItems := []OutputArtifactsItemType{
		OutputArtifactsItemUkis,
		OutputArtifactsItemShim,
		OutputArtifactsItemBootloader,
		OutputArtifactsItemVerityHash,
		OutputArtifactsItemDefault,
	}

	for _, item := range validItems {
		err := item.IsValidOutputItem()
		assert.NoError(t, err, "valid output item should not return an error: %s", item)
	}
}

func TestOutputArtifactsItemTypeIsValidOutputItem_UkiAddons_Fail(t *testing.T) {
	err := OutputArtifactsItemUkiAddons.IsValidOutputItem()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "uki-addons are automatically included with ukis")
}

func TestOutputArtifactsItemTypeIsValidOutputItem_InvalidItem_Fail(t *testing.T) {
	invalidItem := OutputArtifactsItemType("invalidItem")
	err := invalidItem.IsValidOutputItem()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}
