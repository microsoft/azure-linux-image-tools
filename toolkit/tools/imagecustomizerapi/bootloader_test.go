// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootLoaderIsValid(t *testing.T) {
	validBootLoader := BootLoader{
		ResetType: ResetBootLoaderTypeHard,
	}

	err := validBootLoader.IsValid()
	assert.NoError(t, err)
}

func TestBootLoaderIsValidDefault(t *testing.T) {
	defaultBootLoader := BootLoader{
		ResetType: ResetBootLoaderTypeDefault,
	}

	err := defaultBootLoader.IsValid()
	assert.NoError(t, err)
}

func TestBootLoaderIsValidInvalidResetType(t *testing.T) {
	invalidBootLoader := BootLoader{
		ResetType: ResetBootLoaderType("invalid"),
	}

	err := invalidBootLoader.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid resetBootLoaderType value (invalid)")
}
