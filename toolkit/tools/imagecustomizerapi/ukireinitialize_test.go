// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUkiReinitializeIsValid_Invalid(t *testing.T) {
	reinitialize := UkiReinitialize("invalid")
	err := reinitialize.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid uki reinitialize value (invalid): must be one of ['', 'passthrough', 'refresh']")
}

func TestUkiReinitializeIsValid_Typo(t *testing.T) {
	reinitialize := UkiReinitialize("Passthrough")
	err := reinitialize.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid uki reinitialize value (Passthrough)")
}
