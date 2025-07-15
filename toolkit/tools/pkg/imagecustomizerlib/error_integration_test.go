// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

// Test that demonstrates error categorization works end-to-end
func TestValidateConfig_ErrorCategorization(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		baseConfigPath      string
		config              *imagecustomizerapi.Config
		inputImageFile      string
		rpmsSources         []string
		outputImageFile     string
		outputImageFormat   string
		useBaseImageRpmRepos bool
		packageSnapshotTime string
		expectedErrorType   error
	}{
		{
			name:           "missing input image file",
			baseConfigPath: "/tmp/test",
			config: &imagecustomizerapi.Config{
				Input: imagecustomizerapi.Input{},
			},
			inputImageFile:      "",
			rpmsSources:         []string{},
			outputImageFile:     "/tmp/output.raw",
			outputImageFormat:   "raw",
			useBaseImageRpmRepos: false,
			packageSnapshotTime: "",
			expectedErrorType:   nil, // This should return the global error directly
		},
		{
			name:           "missing output image file",
			baseConfigPath: "/tmp/test",
			config: &imagecustomizerapi.Config{
				Input: imagecustomizerapi.Input{},
				Output: imagecustomizerapi.Output{},
			},
			inputImageFile:      "/tmp/input.raw",
			rpmsSources:         []string{},
			outputImageFile:     "",
			outputImageFormat:   "raw",
			useBaseImageRpmRepos: false,
			packageSnapshotTime: "",
			expectedErrorType:   ConfigValidationError, // This will be a dynamic error due to file validation
		},
		{
			name:           "invalid snapshot time with missing preview feature",
			baseConfigPath: "/tmp/test",
			config: &imagecustomizerapi.Config{
				Input: imagecustomizerapi.Input{
					Image: imagecustomizerapi.InputImage{
						Path: "/tmp/input.raw",
					},
				},
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Path:   "/tmp/output.raw",
						Format: imagecustomizerapi.ImageFormatTypeRaw,
					},
				},
				PreviewFeatures: []imagecustomizerapi.PreviewFeature{},
			},
			inputImageFile:      "/tmp/input.raw",
			rpmsSources:         []string{},
			outputImageFile:     "/tmp/output.raw",
			outputImageFormat:   "raw",
			useBaseImageRpmRepos: false,
			packageSnapshotTime: "2024-01-01",
			expectedErrorType:   ConfigValidationError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(ctx, tc.baseConfigPath, tc.config, tc.inputImageFile, tc.rpmsSources,
				tc.outputImageFile, tc.outputImageFormat, tc.useBaseImageRpmRepos, tc.packageSnapshotTime, false)

			assert.NotNil(t, err)

			if tc.expectedErrorType != nil {
				// Check if it's an ImageCustomizerError with the expected type
				var icErr *ImageCustomizerError
				if assert.True(t, errors.As(err, &icErr)) {
					assert.True(t, errors.Is(icErr, tc.expectedErrorType))
				}
			} else {
				// For global errors, check if it's one of our defined global errors
				isGlobalError := errors.Is(err, ErrInputImageFileRequired) ||
					errors.Is(err, ErrOutputImageFileRequired) ||
					errors.Is(err, ErrOutputImageFormatRequired) ||
					errors.Is(err, ErrCannotCustomizePartitionsIso) ||
					errors.Is(err, ErrRpmSourcesRequiredForPackages)

				assert.True(t, isGlobalError, "Expected a global error but got: %v", err)
			}
		})
	}
}

// Test that we can handle wrapped errors correctly
func TestCreateImageCustomizerParameters_ErrorWrapping(t *testing.T) {
	ctx := context.Background()

	// Test with invalid output image format
	ic, err := createImageCustomizerParameters(ctx, "/tmp/build", "/tmp/input.raw", "/tmp/config",
		&imagecustomizerapi.Config{
			Output: imagecustomizerapi.Output{
				Image: imagecustomizerapi.OutputImage{
					Format: imagecustomizerapi.ImageFormatType("invalid-format"),
				},
			},
		}, true, []string{}, "invalid-format", "/tmp/output.raw", "")

	assert.Nil(t, ic)
	assert.NotNil(t, err)

	// Should be an ImageCustomizerError with the image conversion type
	var icErr *ImageCustomizerError
	if assert.True(t, errors.As(err, &icErr)) {
		assert.True(t, errors.Is(icErr, ImageConversionError))
		assert.Contains(t, icErr.Error(), "invalid output image format")
	}
}

// Test that environment variable checking works with the new error handling
func TestCheckEnvironmentVars_WithNewErrorHandling(t *testing.T) {
	// Save original environment
	origHome := os.Getenv("HOME")
	origUser := os.Getenv("USER")
	
	// Restore environment after test
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USER", origUser)
	}()

	// Set correct environment - should not return error
	os.Setenv("HOME", "/root")
	os.Setenv("USER", "root")
	
	err := CheckEnvironmentVars()
	assert.Nil(t, err)

	// Set incorrect environment - should return ImageCustomizerError
	os.Setenv("HOME", "/home/user")
	os.Setenv("USER", "user")
	
	err = CheckEnvironmentVars()
	assert.NotNil(t, err)

	var icErr *ImageCustomizerError
	if assert.True(t, errors.As(err, &icErr)) {
		assert.True(t, errors.Is(icErr, ConfigValidationError))
		assert.Contains(t, icErr.Error(), "tool should be run as root")
	}
}

// Test that we can distinguish between different error types
func TestErrorTypeDistinction(t *testing.T) {
	configErr := NewImageCustomizerError(ConfigValidationError, "config error")
	conversionErr := NewImageCustomizerError(ImageConversionError, "conversion error")
	fsErr := NewImageCustomizerError(FilesystemOperationError, "filesystem error")
	pkgErr := NewImageCustomizerError(PackageManagementError, "package error")
	scriptErr := NewImageCustomizerError(ScriptExecutionError, "script error")
	systemErr := NewImageCustomizerError(InternalSystemError, "system error")

	// Test that each error type is distinct
	assert.True(t, errors.Is(configErr, ConfigValidationError))
	assert.False(t, errors.Is(configErr, ImageConversionError))
	assert.False(t, errors.Is(configErr, FilesystemOperationError))
	assert.False(t, errors.Is(configErr, PackageManagementError))
	assert.False(t, errors.Is(configErr, ScriptExecutionError))
	assert.False(t, errors.Is(configErr, InternalSystemError))

	assert.True(t, errors.Is(conversionErr, ImageConversionError))
	assert.False(t, errors.Is(conversionErr, ConfigValidationError))

	assert.True(t, errors.Is(fsErr, FilesystemOperationError))
	assert.False(t, errors.Is(fsErr, ConfigValidationError))

	assert.True(t, errors.Is(pkgErr, PackageManagementError))
	assert.False(t, errors.Is(pkgErr, ConfigValidationError))

	assert.True(t, errors.Is(scriptErr, ScriptExecutionError))
	assert.False(t, errors.Is(scriptErr, ConfigValidationError))

	assert.True(t, errors.Is(systemErr, InternalSystemError))
	assert.False(t, errors.Is(systemErr, ConfigValidationError))
}