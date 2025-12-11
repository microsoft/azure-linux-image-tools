// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosiCompressionIsValid_DefaultValues(t *testing.T) {
	compression := CosiCompression{}
	err := compression.IsValid()
	assert.NoError(t, err)
}

func TestCosiCompressionIsValid_ValidLevel(t *testing.T) {
	testCases := []struct {
		name  string
		level int
	}{
		{"minimum", MinCosiCompressionLevel},
		{"default", DefaultCosiCompressionLevel},
		{"middle", 15},
		{"ultra", 20},
		{"maximum", MaxCosiCompressionLevel},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compression := CosiCompression{Level: tc.level}
			err := compression.IsValid()
			assert.NoError(t, err)
		})
	}
}

func TestCosiCompressionIsValid_InvalidLevel(t *testing.T) {
	testCases := []struct {
		name  string
		level int
	}{
		{"negative", -1},
		{"too high", 23},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compression := CosiCompression{Level: tc.level}
			err := compression.IsValid()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid 'level' value")
		})
	}
}

func TestCosiCompressionGetLevel(t *testing.T) {
	// Test zero value
	compression := &CosiCompression{}
	assert.Equal(t, DefaultCosiCompressionLevel, compression.GetLevel())

	// Test explicit value
	compression = &CosiCompression{Level: 15}
	assert.Equal(t, 15, compression.GetLevel())
}
