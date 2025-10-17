// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOciImageIsValidSimple(t *testing.T) {
	img := OciImage{
		Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
	}
	assert.NoError(t, img.IsValid())
}

func TestOciImageIsValidWithPlatform(t *testing.T) {
	img := OciImage{
		Uri: "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
		Platform: &OciPlatform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}
	assert.NoError(t, img.IsValid())
}

func TestOciImageIsValidBadUri(t *testing.T) {
	img := OciImage{
		Uri: "mcr.microsoft.com",
		Platform: &OciPlatform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}
	assert.ErrorContains(t, img.IsValid(), "invalid 'uri' field")
}

func TestOciImageParseString(t *testing.T) {
	uri := "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest"
	value := OciImage{}
	err := UnmarshalYaml([]byte(uri), &value)
	assert.NoError(t, err)
	assert.Equal(t, uri, value.Uri)
}

func TestOciImageParseUnknownField(t *testing.T) {
	json := "{\"cat\":\"meow\"}"
	value := OciImage{}
	err := UnmarshalYaml([]byte(json), &value)
	assert.ErrorContains(t, err, "line 1: field cat not found in type OciImage")
}

func TestOciImageParseStruct(t *testing.T) {
	uri := "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest"
	json := "{\"uri\":\"" + uri + "\"}"
	value := OciImage{}
	err := UnmarshalYaml([]byte(json), &value)
	assert.NoError(t, err)
	assert.Equal(t, uri, value.Uri)
}
