// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestInjectFilesOptionsIsValid_ValidOptions_Pass(t *testing.T) {
	options := InjectFilesOptions{}
	err := options.IsValid()
	assert.NoError(t, err)
}

func TestInjectFilesOptionsIsValid_CompressionLevel_Pass(t *testing.T) {
	testCases := []int{1, 9, 15, 22}
	for _, level := range testCases {
		options := InjectFilesOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.NoError(t, err, "level %d should be valid", level)
	}
}

func TestInjectFilesOptionsIsValid_CompressionLevel_Fail(t *testing.T) {
	testCases := []int{-1, 0, 23, 100}
	for _, level := range testCases {
		options := InjectFilesOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.ErrorIs(t, err, imagecustomizerapi.ErrInvalidCosiCompressionLevelArg, "level %d should be invalid", level)
	}
}
