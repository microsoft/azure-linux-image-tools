// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtendedMountIdentifierTypeIsValid(t *testing.T) {
	validCases := []ExtendedMountIdentifierType{
		ExtendedMountIdentifierTypeUuid,
		ExtendedMountIdentifierTypePartUuid,
		ExtendedMountIdentifierTypePartLabel,
		ExtendedMountIdentifierTypeDev,
		ExtendedMountIdentifierTypeDefault,
	}

	for _, validCase := range validCases {
		t.Run(string(validCase), func(t *testing.T) {
			err := validCase.IsValid()
			assert.NoError(t, err)
		})
	}

	// Test invalid case
	invalidCase := ExtendedMountIdentifierType("invalid-type")
	err := invalidCase.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value (invalid-type)")
}
