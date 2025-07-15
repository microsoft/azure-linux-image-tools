// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalErrorTypes(t *testing.T) {
	// Test that all error types are distinct
	errorTypes := []error{
		ConfigValidationError,
		ImageConversionError,
		FilesystemOperationError,
		PackageManagementError,
		ScriptExecutionError,
		InternalSystemError,
		InvalidInputError,
	}

	for i, errType1 := range errorTypes {
		for j, errType2 := range errorTypes {
			if i != j {
				assert.False(t, errors.Is(errType1, errType2), "Error types should be distinct")
			}
		}
	}

	// Test that error types have expected string representations
	assert.Equal(t, "config-validation", ConfigValidationError.Error())
	assert.Equal(t, "image-conversion", ImageConversionError.Error())
	assert.Equal(t, "filesystem-operation", FilesystemOperationError.Error())
	assert.Equal(t, "package-management", PackageManagementError.Error())
	assert.Equal(t, "script-execution", ScriptExecutionError.Error())
	assert.Equal(t, "internal-system", InternalSystemError.Error())
	assert.Equal(t, "invalid-input", InvalidInputError.Error())
}

func TestGlobalErrorMessages(t *testing.T) {
	// Test static error messages
	assert.Equal(t, "input image file must be specified, either via the command line option '--image-file' or in the config file property 'input.image.path'", InputImageFileRequiredError.Error())
	assert.Equal(t, "output image file must be specified, either via the command line option '--output-image-file' or in the config file property 'output.image.path'", OutputImageFileRequiredError.Error())
	assert.Equal(t, "tool should be run as root (e.g. by using sudo)", ToolMustRunAsRootError.Error())
	assert.Equal(t, "the 'uki' preview feature must be enabled to use 'os.uki'", UkiPreviewFeatureRequiredError.Error())
	assert.Equal(t, "'os.bootloader.reset' must be specified if 'storage.disks' is specified", BootLoaderResetRequiredError.Error())
	assert.Equal(t, "'os.bootloader.reset' must be specified if 'storage.resetPartitionsUuidsType' is specified", BootLoaderResetUuidsRequiredError.Error())
	assert.Equal(t, "output image format must be specified, either via the command line option '--output-image-format' or in the config file property 'output.image.format'", OutputImageFormatRequiredError.Error())
	assert.Equal(t, "cannot customize partitions when the input is an iso", CannotCustomizePartitionsIsoError.Error())
	assert.Equal(t, "have packages to install or update but no RPM sources were specified", RpmSourcesRequiredForPackagesError.Error())
}

func TestImageCustomizerError_WithoutCause(t *testing.T) {
	message := "test message"
	err := NewImageCustomizerError(ConfigValidationError, message)

	assert.Equal(t, message, err.Error())
	assert.Equal(t, ConfigValidationError, err.Type)
	assert.Equal(t, message, err.Message)
	assert.Nil(t, err.Cause)
}

func TestImageCustomizerError_WithCause(t *testing.T) {
	message := "test message"
	cause := errors.New("underlying error")
	err := NewImageCustomizerErrorWithCause(ConfigValidationError, message, cause)

	expectedErrorMessage := fmt.Sprintf("%s:\n%v", message, cause)
	assert.Equal(t, expectedErrorMessage, err.Error())
	assert.Equal(t, ConfigValidationError, err.Type)
	assert.Equal(t, message, err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestImageCustomizerError_Unwrap(t *testing.T) {
	// Test without cause
	err1 := NewImageCustomizerError(ConfigValidationError, "test message")
	assert.Nil(t, err1.Unwrap())

	// Test with cause
	cause := errors.New("underlying error")
	err2 := NewImageCustomizerErrorWithCause(ConfigValidationError, "test message", cause)
	assert.Equal(t, cause, err2.Unwrap())
}

func TestImageCustomizerError_Is(t *testing.T) {
	err := NewImageCustomizerError(ConfigValidationError, "test message")

	// Test positive case
	assert.True(t, err.Is(ConfigValidationError))
	assert.True(t, errors.Is(err, ConfigValidationError))

	// Test negative cases
	assert.False(t, err.Is(ImageConversionError))
	assert.False(t, errors.Is(err, ImageConversionError))
	assert.False(t, err.Is(errors.New("random error")))
}

func TestImageCustomizerError_ErrorsIsCompatibility(t *testing.T) {
	// Test that errors.Is() works correctly with ImageCustomizerError
	err1 := NewImageCustomizerError(ConfigValidationError, "config error")
	err2 := NewImageCustomizerError(ImageConversionError, "conversion error")
	
	// Test direct Is() calls
	assert.True(t, errors.Is(err1, ConfigValidationError))
	assert.True(t, errors.Is(err2, ImageConversionError))
	assert.False(t, errors.Is(err1, ImageConversionError))
	assert.False(t, errors.Is(err2, ConfigValidationError))
}

func TestImageCustomizerError_ChainedErrors(t *testing.T) {
	// Test error chaining works correctly
	originalErr := errors.New("original error")
	wrappedErr := NewImageCustomizerErrorWithCause(ConfigValidationError, "wrapped error", originalErr)
	
	// Test that we can unwrap to get to the original error
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.True(t, errors.Is(wrappedErr, ConfigValidationError))
}

func TestImageCustomizerError_MessageFormatting(t *testing.T) {
	// Test that message formatting matches expected patterns
	testCases := []struct {
		name           string
		errorType      error
		message        string
		cause          error
		expectedFormat string
	}{
		{
			name:           "no cause",
			errorType:      ConfigValidationError,
			message:        "simple message",
			cause:          nil,
			expectedFormat: "simple message",
		},
		{
			name:           "with cause",
			errorType:      ConfigValidationError,
			message:        "context message",
			cause:          errors.New("underlying error"),
			expectedFormat: "context message:\nunderlying error",
		},
		{
			name:           "file path in message",
			errorType:      ConfigValidationError,
			message:        "invalid command-line option '--image-file': '/path/to/file'",
			cause:          nil,
			expectedFormat: "invalid command-line option '--image-file': '/path/to/file'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err *ImageCustomizerError
			if tc.cause != nil {
				err = NewImageCustomizerErrorWithCause(tc.errorType, tc.message, tc.cause)
			} else {
				err = NewImageCustomizerError(tc.errorType, tc.message)
			}
			
			assert.Equal(t, tc.expectedFormat, err.Error())
		})
	}
}

func TestImageCustomizerError_PreviewFeaturePattern(t *testing.T) {
	// Test the preview feature error pattern
	previewFeature := "preview-feature-name"
	message := fmt.Sprintf("the '%s' preview feature must be enabled to use 'some.config'", previewFeature)
	err := NewImageCustomizerError(ConfigValidationError, message)

	expectedMessage := "the 'preview-feature-name' preview feature must be enabled to use 'some.config'"
	assert.Equal(t, expectedMessage, err.Error())
	assert.True(t, errors.Is(err, ConfigValidationError))
}

func TestImageCustomizerError_ErrorWrappingPattern(t *testing.T) {
	// Test the error wrapping pattern commonly used in the codebase
	originalErr := errors.New("file not found")
	filePath := "/path/to/config.yaml"
	
	message := fmt.Sprintf("invalid config file property 'input.image.path': '%s'", filePath)
	err := NewImageCustomizerErrorWithCause(ConfigValidationError, message, originalErr)

	expectedMessage := fmt.Sprintf("invalid config file property 'input.image.path': '%s':\nfile not found", filePath)
	assert.Equal(t, expectedMessage, err.Error())
	assert.True(t, errors.Is(err, ConfigValidationError))
	assert.True(t, errors.Is(err, originalErr))
}