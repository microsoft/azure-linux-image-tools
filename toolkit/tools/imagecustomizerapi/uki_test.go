// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUkiIsValidWithCreate(t *testing.T) {
	validUki := Uki{
		Mode: UkiModeCreate,
	}

	err := validUki.IsValid()
	assert.NoError(t, err)
}

func TestUkiIsValidWithPassthrough(t *testing.T) {
	validUki := Uki{
		Mode: UkiModePassthrough,
	}

	err := validUki.IsValid()
	assert.NoError(t, err)
}

func TestUkiIsValidWithUnspecified(t *testing.T) {
	validUki := Uki{
		Mode: UkiModeUnspecified,
	}

	err := validUki.IsValid()
	assert.NoError(t, err)
}

func TestUkiIsValidInvalidMode(t *testing.T) {
	invalidUki := Uki{
		Mode: UkiMode("invalid-mode"),
	}

	err := invalidUki.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid uki mode value (invalid-mode)")
}
