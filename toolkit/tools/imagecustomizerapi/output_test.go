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
		Artifacts: &Artifacts{
			Items: []OutputArtifactsItemType{"invalidItem"},
			Path:  "/valid/path",
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid item value")
}

func TestOutputIsValid_ValidArtifactsIsValid(t *testing.T) {
	output := Output{
		Artifacts: &Artifacts{
			Items: []OutputArtifactsItemType{
				OutputArtifactsItemUkis,
				OutputArtifactsItemShim,
				OutputArtifactsItemSystemdBoot,
			},
			Path: "/valid/path",
		},
	}
	err := output.IsValid()
	assert.NoError(t, err)
}

func TestOutputIsValid_InvalidArtifactsPathCombination(t *testing.T) {
	output := Output{
		Artifacts: &Artifacts{
			Items: []OutputArtifactsItemType{OutputArtifactsItemUkis},
			Path:  "",
		},
	}
	err := output.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'items' and 'path' must both be specified and non-empty")
}
