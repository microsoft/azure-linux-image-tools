// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactsIsValid_InvalidItemIsInvalid(t *testing.T) {
	artifacts := Artifacts{
		Items: []OutputArtifactsItemType{"invalidItem"},
		Path:  "/valid/path",
	}
	err := artifacts.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}

func TestArtifactsIsValid_ValidArtifactsIsValid(t *testing.T) {
	artifacts := Artifacts{
		Items: []OutputArtifactsItemType{
			OutputArtifactsItemUkis,
			OutputArtifactsItemShim,
			OutputArtifactsItemSystemdBoot,
		},
		Path: "/valid/path",
	}
	err := artifacts.IsValid()
	assert.NoError(t, err)
}

func TestArtifactsIsValid_InvalidItemsPathCombination(t *testing.T) {
	artifacts := Artifacts{
		Items: []OutputArtifactsItemType{OutputArtifactsItemUkis},
		Path:  "",
	}
	err := artifacts.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'items' and 'path' must both be specified and non-empty")
}
