// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdTypeIsValid(t *testing.T) {
	err := IdentifiedPartitionTypePartLabel.IsValid()
	assert.NoError(t, err)
}

func TestIdTypeIsValidBadValue(t *testing.T) {
	err := IdentifiedPartitionType("bad").IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid value (bad)")
}
