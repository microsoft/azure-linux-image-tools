// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUkiModeIsValidCreate(t *testing.T) {
	mode := UkiModeCreate
	err := mode.IsValid()
	assert.NoError(t, err)
}

func TestUkiModeIsValidPassthrough(t *testing.T) {
	mode := UkiModePassthrough
	err := mode.IsValid()
	assert.NoError(t, err)
}

func TestUkiModeIsValidUnspecified(t *testing.T) {
	mode := UkiModeUnspecified
	err := mode.IsValid()
	assert.NoError(t, err)
}

func TestUkiModeIsValidInvalid(t *testing.T) {
	mode := UkiMode("invalid-mode")
	err := mode.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid uki mode value (invalid-mode)")
}

func TestUkiModeIsValidTypo(t *testing.T) {
	mode := UkiMode("Create")
	err := mode.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid uki mode value (Create)")
}
