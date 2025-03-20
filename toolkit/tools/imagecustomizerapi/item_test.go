// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItemIsValid_ValidItems(t *testing.T) {
	validItems := []Item{ItemUkis, ItemShim, ItemSystemdBoot, ItemDefault}

	for _, item := range validItems {
		err := item.IsValid()
		assert.NoError(t, err, "valid item should not return an error: %s", item)
	}
}

func TestItemIsValid_InvalidItem(t *testing.T) {
	invalidItem := Item("invalidItem")
	err := invalidItem.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}
