// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package vhdutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestGetVhdFileSizeCalcTypeDiskGeometryDynamic(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeDiskGeometryDynamic", VhdFileSizeCalcTypeDiskGeometry,
		[]string{"-f", "vpc", "-o", "force_size=off,subformat=dynamic"})
}

func TestGetVhdFileSizeCalcTypeDiskGeometryFixed(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeDiskGeometryFixed", VhdFileSizeCalcTypeDiskGeometry,
		[]string{"-f", "vpc", "-o", "force_size=off,subformat=fixed"})
}

func TestGetVhdFileSizeCalcTypeCurrentSizeDynamic(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeCurrentSizeDynamic", VhdFileSizeCalcTypeCurrentSize,
		[]string{"-f", "vpc", "-o", "force_size=on,subformat=dynamic"})
}

func TestGetVhdFileSizeCalcTypeCurrentSizeFixed(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeCurrentSizeFixed", VhdFileSizeCalcTypeCurrentSize,
		[]string{"-f", "vpc", "-o", "force_size=on,subformat=fixed"})
}

func TestGetVhdFileSizeCalcTypeNoneVhdx(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeNoneVhdx", VhdFileSizeCalcTypeNone,
		[]string{"-f", "vhdx"})
}

func TestGetVhdFileSizeCalcTypeNoneQcow2(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeNoneQcow2", VhdFileSizeCalcTypeNone,
		[]string{"-f", "qcow2"})
}

func TestGetVhdFileSizeCalcTypeNoneRaw(t *testing.T) {
	testGetVhdFileSizeCalcTypeHelper(t, "TestGetVhdFileSizeCalcTypeNoneRaw", VhdFileSizeCalcTypeNone,
		[]string{"-f", "raw"})
}

func testGetVhdFileSizeCalcTypeHelper(t *testing.T, testName string, expectedVhdFileSizeCalcType VhdFileSizeCalcType, qemuImgArgs []string) {
	qemuimgExists, err := file.CommandExists("qemu-img")
	assert.NoError(t, err)
	if !qemuimgExists {
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

	vhdType, err := GetVhdFileSizeCalcType(testVhdFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedVhdFileSizeCalcType, vhdType)
}
