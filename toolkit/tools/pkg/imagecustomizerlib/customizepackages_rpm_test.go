// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRpmQueryOutput_ValidLines(t *testing.T) {
	input := "bash 5.2.26 3.azl3 x86_64\nsystemd 255 12.azl3 x86_64\n"
	packages, err := parseRpmQueryOutput(input)
	require.NoError(t, err)
	assert.Len(t, packages, 2)
	assert.Equal(t, "bash", packages[0].Name)
	assert.Equal(t, "5.2.26", packages[0].Version)
	assert.Equal(t, "3.azl3", packages[0].Release)
	assert.Equal(t, "x86_64", packages[0].Arch)
	assert.Equal(t, "systemd", packages[1].Name)
}

func TestParseRpmQueryOutput_EmptyOutput(t *testing.T) {
	packages, err := parseRpmQueryOutput("")
	require.NoError(t, err)
	assert.Nil(t, packages)
}

func TestParseRpmQueryOutput_WhitespaceOnly(t *testing.T) {
	packages, err := parseRpmQueryOutput("  \n  \n")
	require.NoError(t, err)
	assert.Nil(t, packages)
}

func TestParseRpmQueryOutput_MalformedLine(t *testing.T) {
	input := "bash 5.2.26 3.azl3 x86_64\nbadline\n"
	_, err := parseRpmQueryOutput(input)
	assert.ErrorContains(t, err, "malformed RPM line")
}

func TestParseRpmQueryOutput_SinglePackage(t *testing.T) {
	input := "kernel 6.6.78 1.azl3 aarch64"
	packages, err := parseRpmQueryOutput(input)
	require.NoError(t, err)
	assert.Len(t, packages, 1)
	assert.Equal(t, "kernel", packages[0].Name)
	assert.Equal(t, "aarch64", packages[0].Arch)
}

func TestAclDetectBootloaderType(t *testing.T) {
	handler := newAclDistroHandler()
	// ACL always returns systemd-boot without needing a real chroot.
	bootloaderType, err := handler.DetectBootloaderType(nil)
	require.NoError(t, err)
	assert.Equal(t, BootloaderTypeSystemdBoot, bootloaderType)
}

func TestAclValidateUkiDependencies(t *testing.T) {
	handler := newAclDistroHandler()
	err := handler.ValidateUkiDependencies(nil)
	require.NoError(t, err)
}
