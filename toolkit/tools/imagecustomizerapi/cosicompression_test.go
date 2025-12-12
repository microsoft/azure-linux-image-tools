// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosiCompressionIsValid_Empty(t *testing.T) {
	compression := CosiCompression{}
	err := compression.IsValid()
	assert.NoError(t, err)
}

func TestCosiCompressionIsValid_ValidLevel(t *testing.T) {
	testCases := []int{1, 9, 15, 22}
	for _, level := range testCases {
		compression := CosiCompression{Level: &level}
		err := compression.IsValid()
		assert.NoError(t, err)
	}
}

func TestCosiCompressionIsValid_InvalidLevel(t *testing.T) {
	testCases := []int{-1, 0, 23, 100}
	for _, level := range testCases {
		compression := CosiCompression{Level: &level}
		err := compression.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid 'level' value")
	}
}
