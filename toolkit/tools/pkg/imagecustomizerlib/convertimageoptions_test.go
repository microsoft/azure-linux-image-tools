// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
)

func TestConvertImageOptions_IsValid_Success(t *testing.T) {
	options := ConvertImageOptions{
		BuildDir:          "/tmp/build",
		InputImageFile:    "/tmp/input.vhdx",
		OutputImageFile:   "/tmp/output.cosi",
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeCosi,
	}

	err := options.IsValid()
	assert.NoError(t, err)
}

func TestConvertImageOptions_IsValid_NoBuildDir(t *testing.T) {
	// BuildDir is optional in IsValid() - it's validated at runtime in ConvertImage()
	// based on the output format (required only for COSI/bare-metal-image).
	options := ConvertImageOptions{
		InputImageFile:    "/tmp/input.vhdx",
		OutputImageFile:   "/tmp/output.vhdx",
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhdx,
	}

	err := options.IsValid()
	assert.NoError(t, err)
}

func TestConvertImageOptions_IsValid_MissingInputFile(t *testing.T) {
	options := ConvertImageOptions{
		BuildDir:          "/tmp/build",
		OutputImageFile:   "/tmp/output.cosi",
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeCosi,
	}

	err := options.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input image file must be specified")
}

func TestConvertImageOptions_IsValid_MissingOutputFile(t *testing.T) {
	options := ConvertImageOptions{
		BuildDir:          "/tmp/build",
		InputImageFile:    "/tmp/input.vhdx",
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeCosi,
	}

	err := options.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "output image file must be specified")
}

func TestConvertImageOptions_IsValid_InvalidOutputFormat(t *testing.T) {
	options := ConvertImageOptions{
		BuildDir:          "/tmp/build",
		InputImageFile:    "/tmp/input.vhdx",
		OutputImageFile:   "/tmp/output.cosi",
		OutputImageFormat: "invalid-format",
	}

	err := options.IsValid()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidOutputFormat)
}

func TestConvertImageOptions_IsValid_InvalidCosiCompressionLevel(t *testing.T) {
	invalidLevels := []int{0, -1, 23, 100}

	for _, level := range invalidLevels {
		options := ConvertImageOptions{
			BuildDir:             "/tmp/build",
			InputImageFile:       "/tmp/input.vhdx",
			OutputImageFile:      "/tmp/output.cosi",
			OutputImageFormat:    imagecustomizerapi.ImageFormatTypeCosi,
			CosiCompressionLevel: ptrutils.PtrTo(level),
		}

		err := options.IsValid()
		assert.ErrorIs(t, err, imagecustomizerapi.ErrInvalidCosiCompressionLevelArg, "level %d should be invalid", level)
	}
}

func TestConvertImageOptions_IsValid_ValidCosiCompressionLevel(t *testing.T) {
	validLevels := []int{
		imagecustomizerapi.MinCosiCompressionLevel,
		imagecustomizerapi.DefaultCosiCompressionLevel,
		imagecustomizerapi.MaxCosiCompressionLevel,
		10,
		15,
	}

	for _, level := range validLevels {
		options := ConvertImageOptions{
			BuildDir:             "/tmp/build",
			InputImageFile:       "/tmp/input.vhdx",
			OutputImageFile:      "/tmp/output.cosi",
			OutputImageFormat:    imagecustomizerapi.ImageFormatTypeCosi,
			CosiCompressionLevel: ptrutils.PtrTo(level),
		}

		err := options.IsValid()
		assert.NoError(t, err, "level %d should be valid", level)
	}
}
