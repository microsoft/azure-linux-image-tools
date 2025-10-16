// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOciPlatformParseStringOsOnly(t *testing.T) {
	uri := "linux"
	value := OciPlatform{}
	err := UnmarshalYaml([]byte(uri), &value)
	assert.NoError(t, err)
	assert.Equal(t, "linux", value.OS)
}

func TestOciPlatformParseStringOsAndArch(t *testing.T) {
	uri := "linux/amd64"
	value := OciPlatform{}
	err := UnmarshalYaml([]byte(uri), &value)
	assert.NoError(t, err)
	assert.Equal(t, "linux", value.OS)
	assert.Equal(t, "amd64", value.Architecture)
}

func TestOciPlatformParseStringBad(t *testing.T) {
	uri := "linux/amd64/cat/dog/elephant"
	value := OciPlatform{}
	err := UnmarshalYaml([]byte(uri), &value)
	assert.ErrorContains(t, err, "invalid OCI platform string")
}

func TestOciPlatformParseUnknownField(t *testing.T) {
	json := "{\"cat\":\"meow\"}"
	value := OciPlatform{}
	err := UnmarshalYaml([]byte(json), &value)
	assert.ErrorContains(t, err, "line 1: field cat not found in type OciPlatform")
}

func TestOciPlatformParseStruct(t *testing.T) {
	json := "{\"os\": \"linux\", \"architecture\":\"amd64\"}"
	value := OciPlatform{}
	err := UnmarshalYaml([]byte(json), &value)
	assert.NoError(t, err)
	assert.Equal(t, "linux", value.OS)
	assert.Equal(t, "amd64", value.Architecture)
}
