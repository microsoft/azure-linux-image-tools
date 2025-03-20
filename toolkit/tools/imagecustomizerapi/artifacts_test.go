// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactsIsValid_EmptyIsValid(t *testing.T) {
	artifacts := Artifacts{}
	err := artifacts.IsValid()
	assert.NoError(t, err)
}

func TestArtifactsIsValid_InvalidItemIsInvalid(t *testing.T) {
	artifacts := Artifacts{
		Items: []Item{"invalidItem"},
		Path:  "/valid/path",
	}
	err := artifacts.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}

func TestArtifactsIsValid_ValidArtifactsIsValid(t *testing.T) {
	artifacts := Artifacts{
		Items: []Item{ItemUkis, ItemShim, ItemSystemdBoot},
		Path:  "/valid/path",
	}
	err := artifacts.IsValid()
	assert.NoError(t, err)
}

func TestArtifactsIsValid_InvalidItemsPathCombination(t *testing.T) {
	artifacts := Artifacts{
		Items: []Item{ItemUkis},
		Path:  "",
	}
	err := artifacts.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'items' and 'path' should either both be provided or neither")
}

func TestArtifactsIsValid_InvalidPath(t *testing.T) {
	artifacts := Artifacts{
		Items: []Item{ItemUkis},
		Path:  "invalid_path\\with\\backslashes",
	}
	err := artifacts.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'path' field")
}
