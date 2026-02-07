// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateResourceTypeIsValid_All_Pass(t *testing.T) {
	err := ValidateResourceTypeAll.IsValid()
	assert.NoError(t, err)
}

func TestValidateResourceTypeIsValid_InvalidType_Fail(t *testing.T) {
	err := ValidateResourceType("invalid-type").IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid validate resource type")
}
