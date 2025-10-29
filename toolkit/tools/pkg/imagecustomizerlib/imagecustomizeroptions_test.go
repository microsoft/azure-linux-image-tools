// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseInputImageOciValid(t *testing.T) {
	inputImage, err := parseInputImage("oci:mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest")
	assert.NoError(t, err)
	if !assert.NotNil(t, inputImage.Oci) {
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
