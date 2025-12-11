// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosiConfigIsValid_Empty(t *testing.T) {
	config := CosiConfig{}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestCosiConfigIsValid_ValidCompression(t *testing.T) {
	config := CosiConfig{
		Compression: &CosiCompression{
			Level: 15,
		},
	}
	err := config.IsValid()
	assert.NoError(t, err)
}

func TestCosiConfigIsValid_InvalidCompression(t *testing.T) {
	config := CosiConfig{
		Compression: &CosiCompression{
			Level: 30,
		},
	}
	err := config.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'compression' value")
	assert.ErrorContains(t, err, "invalid 'level' value")
}
