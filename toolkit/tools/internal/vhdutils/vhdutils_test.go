// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package vhdutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestGetVhdFileTypeDiskGeometryDynamic(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeDiskGeometryDynamic", VhdFileTypeDiskGeometry,
		[]string{"-f", "vpc", "-o", "force_size=off,subformat=dynamic"})
}

func TestGetVhdFileTypeDiskGeometryFixed(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeDiskGeometryFixed", VhdFileTypeDiskGeometry,
		[]string{"-f", "vpc", "-o", "force_size=off,subformat=fixed"})
}

func TestGetVhdFileTypeCurrentSizeDynamic(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeCurrentSizeDynamic", VhdFileTypeCurrentSize,
		[]string{"-f", "vpc", "-o", "force_size=on,subformat=dynamic"})
}

func TestGetVhdFileTypeCurrentSizeFixed(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeCurrentSizeFixed", VhdFileTypeCurrentSize,
		[]string{"-f", "vpc", "-o", "force_size=on,subformat=fixed"})
}

func TestGetVhdFileTypeNoneVhdx(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeNoneVhdx", VhdFileTypeNone,
		[]string{"-f", "vhdx"})
}

func TestGetVhdFileTypeNoneRaw(t *testing.T) {
	testGetVhdFileTypeHelper(t, "TestGetVhdFileTypeNoneRaw", VhdFileTypeNone,
		[]string{"-f", "raw"})
}

func testGetVhdFileTypeHelper(t *testing.T, testName string, expectedVhdFileType VhdFileType, qemuImgArgs []string) {
	ukifyExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'qemu-img' command is not available")
	}

	testTempDir := filepath.Join(testsTempDir, testName)
	testVhdFile := filepath.Join(testTempDir, "test.vhd")

	err = os.MkdirAll(testTempDir, os.ModePerm)
	assert.NoError(t, err)

	args := []string{"create", testVhdFile, "1M"}
	args = append(args, qemuImgArgs...)

	err = shell.ExecuteLive(true, "qemu-img", args...)
	assert.NoError(t, err)

	vhdType, err := GetVhdFileType(testVhdFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVhdFileType, vhdType)
}
