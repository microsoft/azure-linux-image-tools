// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseConfigIsValidNoPath(t *testing.T) {
	base := BaseConfig{
		Path: "",
	}
	err := base.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not be empty or whitespace")
}

func TestBaseConfigIsValidWhitespaces(t *testing.T) {
	base := BaseConfig{
		Path: "   ",
	}
	err := base.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not be empty or whitespace")
}
