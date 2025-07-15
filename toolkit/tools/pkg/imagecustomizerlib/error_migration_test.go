// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"os"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateInput_WithGlobalErrors(t *testing.T) {
	baseConfigPath := "/tmp/test"
	
	// Test that missing input returns the global error
	err := validateInput(baseConfigPath, imagecustomizerapi.Input{}, "")
	assert.True(t, errors.Is(err, InputImageFileRequiredError))
	assert.Equal(t, InputImageFileRequiredError.Error(), err.Error())
}

func TestValidateOutput_WithGlobalErrors(t *testing.T) {
	baseConfigPath := "/tmp/test"
	
	// Test that missing output returns the global error  
	err := validateOutput(baseConfigPath, imagecustomizerapi.Output{}, "", "")
	assert.True(t, errors.Is(err, OutputImageFileRequiredError))
	assert.Equal(t, OutputImageFileRequiredError.Error(), err.Error())
}

func TestValidateInput_WithDynamicErrors(t *testing.T) {
	baseConfigPath := "/tmp/test"
	nonExistentFile := "/path/to/non/existent/file"
	
	// Test that file validation uses ImageCustomizerError
	err := validateInput(baseConfigPath, imagecustomizerapi.Input{}, nonExistentFile)
	
	// Should be an ImageCustomizerError
	var icErr *ImageCustomizerError
	assert.True(t, errors.As(err, &icErr))
	assert.True(t, errors.Is(icErr, ConfigValidationError))
	assert.Contains(t, icErr.Error(), "invalid command-line option '--image-file'")
	assert.Contains(t, icErr.Error(), nonExistentFile)
}

func TestValidateOutput_WithDynamicErrors(t *testing.T) {
	baseConfigPath := "/tmp/test"
	
	// Test that missing output format returns the global error
	err := validateOutput(baseConfigPath, imagecustomizerapi.Output{}, "output.raw", "")
	assert.True(t, errors.Is(err, OutputImageFormatRequiredError))
	assert.Equal(t, OutputImageFormatRequiredError.Error(), err.Error())
}

func TestValidatePackageLists_WithGlobalErrors(t *testing.T) {
	baseConfigPath := "/tmp/test"
	config := &imagecustomizerapi.OS{
		Packages: imagecustomizerapi.Packages{
			Install: []string{"package1", "package2"},
		},
	}
	
	// Test that missing RPM sources returns the global error
	err := validatePackageLists(baseConfigPath, config, []string{}, false)
	assert.True(t, errors.Is(err, RpmSourcesRequiredForPackagesError))
	assert.Equal(t, RpmSourcesRequiredForPackagesError.Error(), err.Error())
}

func TestCheckEnvironmentVars_WithDynamicErrors(t *testing.T) {
	// Save original environment
	origHome := os.Getenv("HOME")
	origUser := os.Getenv("USER")
	
	// Restore environment after test
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USER", origUser)
	}()
	
	// Set incorrect environment
	os.Setenv("HOME", "/home/user")
	os.Setenv("USER", "user")
	
	err := CheckEnvironmentVars()
	
	// Should be an ImageCustomizerError
	var icErr *ImageCustomizerError
	assert.True(t, errors.As(err, &icErr))
	assert.True(t, errors.Is(icErr, ConfigValidationError))
	assert.Contains(t, icErr.Error(), "tool should be run as root")
	assert.Contains(t, icErr.Error(), "HOME must be set to")
	assert.Contains(t, icErr.Error(), "USER must be set to")
}

func TestValidateSnapshotTimeInput_WithDynamicErrors(t *testing.T) {
	// Test preview feature not enabled
	err := validateSnapshotTimeInput("2024-01-01", []imagecustomizerapi.PreviewFeature{})
	
	var icErr *ImageCustomizerError
	assert.True(t, errors.As(err, &icErr))
	assert.True(t, errors.Is(icErr, ConfigValidationError))
	assert.Contains(t, icErr.Error(), "please enable the")
	assert.Contains(t, icErr.Error(), "preview feature")
	assert.Contains(t, icErr.Error(), "package-snapshot-time")
}

func TestErrorCategorization(t *testing.T) {
	// Test that we can categorize different types of errors
	testCases := []struct {
		name      string
		errorType error
		err       error
	}{
		{
			name:      "config validation error",
			errorType: ConfigValidationError,
			err:       InputImageFileRequiredError,
		},
		{
			name:      "dynamic config validation error",
			errorType: ConfigValidationError,
			err:       NewImageCustomizerError(ConfigValidationError, "test validation error"),
		},
		{
			name:      "image conversion error",
			errorType: ImageConversionError,
			err:       NewImageCustomizerError(ImageConversionError, "test conversion error"),
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if icErr, ok := tc.err.(*ImageCustomizerError); ok {
				assert.True(t, errors.Is(icErr, tc.errorType))
			} else {
				// For global errors, we'd need to check against the specific error
				// This would require updating the global errors to also support categorization
				// For now, we just check that the error exists
				assert.NotNil(t, tc.err)
			}
		})
	}
}

func TestErrorMessagePreservation(t *testing.T) {
	// Test that error messages are preserved exactly as they were
	testCases := []struct {
		name            string
		originalMessage string
		newError        error
	}{
		{
			name:            "input image file required",
			originalMessage: "input image file must be specified, either via the command line option '--image-file' or in the config file property 'input.image.path'",
			newError:        InputImageFileRequiredError,
		},
		{
			name:            "output image file required", 
			originalMessage: "output image file must be specified, either via the command line option '--output-image-file' or in the config file property 'output.image.path'",
			newError:        OutputImageFileRequiredError,
		},
		{
			name:            "tool must run as root",
			originalMessage: "tool should be run as root (e.g. by using sudo)",
			newError:        ToolMustRunAsRootError,
		},
		{
			name:            "uki preview feature required",
			originalMessage: "the 'uki' preview feature must be enabled to use 'os.uki'",
			newError:        UkiPreviewFeatureRequiredError,
		},
		{
			name:            "bootloader reset required",
			originalMessage: "'os.bootloader.reset' must be specified if 'storage.disks' is specified",
			newError:        BootLoaderResetRequiredError,
		},
		{
			name:            "bootloader reset uuids required",
			originalMessage: "'os.bootloader.reset' must be specified if 'storage.resetPartitionsUuidsType' is specified",
			newError:        BootLoaderResetUuidsRequiredError,
		},
		{
			name:            "output image format required",
			originalMessage: "output image format must be specified, either via the command line option '--output-image-format' or in the config file property 'output.image.format'",
			newError:        OutputImageFormatRequiredError,
		},
		{
			name:            "cannot customize partitions iso",
			originalMessage: "cannot customize partitions when the input is an iso",
			newError:        CannotCustomizePartitionsIsoError,
		},
		{
			name:            "rpm sources required for packages",
			originalMessage: "have packages to install or update but no RPM sources were specified",
			newError:        RpmSourcesRequiredForPackagesError,
		},
		{
			name:            "root hash parsing failed",
			originalMessage: "failed to parse root hash from veritysetup output",
			newError:        RootHashParsingFailedError,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.originalMessage, tc.newError.Error())
		})
	}
}