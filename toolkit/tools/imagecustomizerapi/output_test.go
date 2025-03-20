// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputIsValid_EmptyIsValid(t *testing.T) {
	output := Output{}
	err := output.IsValid()
	assert.NoError(t, err)
}

func TestOutputIsValid_InvalidImageIsInvalid(t *testing.T) {
	output := Output{
		Image: OutputImage{
			Format: ImageFormatType("xxx"),
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'image' field")
}

func TestOutputIsValid_InvalidArtifactsIsInvalid(t *testing.T) {
	output := Output{
		Artifacts: Artifacts{
			Items: []Item{"invalidItem"},
			Path:  "/valid/path",
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}

func TestOutputIsValid_ValidArtifactsIsValid(t *testing.T) {
	output := Output{
		Artifacts: Artifacts{
			Items: []Item{ItemUkis, ItemShim, ItemSystemdBoot},
			Path:  "/valid/path",
		},
	}
	err := output.IsValid()
	assert.NoError(t, err)
}

func TestOutputIsValid_InvalidArtifactsPathCombination(t *testing.T) {
	output := Output{
		Artifacts: Artifacts{
			Items: []Item{ItemUkis},
			Path:  "",
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'items' and 'path' should either both be provided or neither")
}

func TestOutputIsValid_InvalidPath(t *testing.T) {
	output := Output{
		Artifacts: Artifacts{
			Items: []Item{ItemUkis},
			Path:  "invalid_path\\with\\backslashes",
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'path' field")
}
