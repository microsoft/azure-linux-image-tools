// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageHistoryIsValidValid(t *testing.T) {
	err := ImageHistoryNone.IsValid()
	assert.NoError(t, err)
}

func TestImageHistoryIsValidInvalid(t *testing.T) {
	err := ImageHistory("aaa").IsValid()
	assert.ErrorContains(t, err, "invalid imageHistory value (aaa)")
}
