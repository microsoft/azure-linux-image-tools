// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestParseInputImageOciValid(t *testing.T) {
	inputImage, err := parseInputImage("oci:mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest")
	assert.NoError(t, err)
	if assert.NotNil(t, inputImage.Oci) {
		assert.Equal(t, "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest", inputImage.Oci.Uri)
	}
}

func TestParseInputImageResourceTypeMissing(t *testing.T) {
	_, err := parseInputImage("oci")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "resource type not found")
}

func TestParseInputImageOciUriMissing(t *testing.T) {
	_, err := parseInputImage("oci:")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "invalid 'uri' field (uri='')")
}

func TestParseInputImageOciUriBad(t *testing.T) {
	_, err := parseInputImage("oci:mcr.microsoft.com")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "invalid reference: missing registry or repository")
}

func TestOptionsIsValid_ValidOptions_Pass(t *testing.T) {
	options := ImageCustomizerOptions{}
	err := options.IsValid()
	assert.NoError(t, err, "nil level should be valid")
}

func TestOptionsIsValid_CosiCompressionLevel_Pass(t *testing.T) {
	testCases := []int{1, 9, 15, 22}
	for _, level := range testCases {
		options := ImageCustomizerOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.NoError(t, err, "level %d should be valid", level)
	}
}

func TestOptionsIsValid_CosiCompressionLevel_Fail(t *testing.T) {
	testCases := []int{-1, 0, 23, 100}
	for _, level := range testCases {
		options := ImageCustomizerOptions{
			CosiCompressionLevel: &level,
		}
		err := options.IsValid()
		assert.ErrorIs(t, err, imagecustomizerapi.ErrInvalidCosiCompressionLevelArg, "level %d should be invalid", level)
	}
}
