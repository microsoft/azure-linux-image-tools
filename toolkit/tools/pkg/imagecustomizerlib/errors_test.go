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
		ErrInvalidInput,
		ErrImageConversion,
		ErrFilesystemOperation,
		ErrPackageManagement,
		ErrScriptExecution,
		ErrInternalSystem,
	}

	for i, errType1 := range errorTypes {
		for j, errType2 := range errorTypes {
			if i != j {
				assert.False(t, errors.Is(errType1, errType2), "Error types should be distinct")
			}
		}
	}

	// Test that error types have expected string representations
	assert.Equal(t, "invalid-input", ErrInvalidInput.Error())
	assert.Equal(t, "image-conversion", ErrImageConversion.Error())
	assert.Equal(t, "filesystem-operation", ErrFilesystemOperation.Error())
	assert.Equal(t, "package-management", ErrPackageManagement.Error())
	assert.Equal(t, "script-execution", ErrScriptExecution.Error())
	assert.Equal(t, "internal-system", ErrInternalSystem.Error())
}

func TestImageCustomizerError_WithoutCause(t *testing.T) {
	message := "test message"
	err := NewImageCustomizerError(ErrInvalidInput, message)

	assert.Equal(t, message, err.Error())
	assert.Equal(t, ErrInvalidInput, err.Type)
	assert.Equal(t, message, err.Message)
	assert.Nil(t, err.Cause)
}

func TestImageCustomizerError_WithCause(t *testing.T) {
	message := "test message"
	cause := errors.New("underlying error")
	err := NewImageCustomizerErrorWithCause(ErrInvalidInput, message, cause)

	expectedErrorMessage := fmt.Sprintf("%s:\n%v", message, cause)
	assert.Equal(t, expectedErrorMessage, err.Error())
	assert.Equal(t, ErrInvalidInput, err.Type)
	assert.Equal(t, message, err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestImageCustomizerError_Unwrap(t *testing.T) {
	// Test without cause
	err1 := NewImageCustomizerError(ErrInvalidInput, "test message")
	assert.Nil(t, err1.Unwrap())

	// Test with cause
	cause := errors.New("underlying error")
	err2 := NewImageCustomizerErrorWithCause(ErrInvalidInput, "test message", cause)
	assert.Equal(t, cause, err2.Unwrap())
}

func TestImageCustomizerError_Is(t *testing.T) {
	err := NewImageCustomizerError(ErrInvalidInput, "test message")

	// Test positive case
	assert.True(t, err.Is(ErrInvalidInput))
	assert.True(t, errors.Is(err, ErrInvalidInput))

	// Test negative cases
	assert.False(t, err.Is(ErrImageConversion))
	assert.False(t, errors.Is(err, ErrImageConversion))
	assert.False(t, err.Is(errors.New("random error")))
}

func TestImageCustomizerError_ErrorsIsCompatibility(t *testing.T) {
	// Test that errors.Is() works correctly with ImageCustomizerError
	err1 := NewImageCustomizerError(ErrInvalidInput, "config error")
	err2 := NewImageCustomizerError(ErrImageConversion, "conversion error")
	
	// Test direct Is() calls
	assert.True(t, errors.Is(err1, ErrInvalidInput))
	assert.True(t, errors.Is(err2, ErrImageConversion))
	assert.False(t, errors.Is(err1, ErrImageConversion))
	assert.False(t, errors.Is(err2, ErrInvalidInput))
}

func TestImageCustomizerError_ChainedErrors(t *testing.T) {
	// Test error chaining works correctly
	originalErr := errors.New("original error")
	wrappedErr := NewImageCustomizerErrorWithCause(ErrInvalidInput, "wrapped error", originalErr)
	
	// Test that we can unwrap to get to the original error
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.True(t, errors.Is(wrappedErr, ErrInvalidInput))
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
			errorType:      ErrInvalidInput,
			message:        "simple message",
			cause:          nil,
			expectedFormat: "simple message",
		},
		{
			name:           "with cause",
			errorType:      ErrInvalidInput,
			message:        "context message",
			cause:          errors.New("underlying error"),
			expectedFormat: "context message:\nunderlying error",
		},
		{
			name:           "file path in message",
			errorType:      ErrInvalidInput,
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
	err := NewImageCustomizerError(ErrInvalidInput, message)

	expectedMessage := "the 'preview-feature-name' preview feature must be enabled to use 'some.config'"
	assert.Equal(t, expectedMessage, err.Error())
	assert.True(t, errors.Is(err, ErrInvalidInput))
}

func TestImageCustomizerError_ErrorWrappingPattern(t *testing.T) {
	// Test the error wrapping pattern commonly used in the codebase
	originalErr := errors.New("file not found")
	filePath := "/path/to/config.yaml"
	
	message := fmt.Sprintf("invalid config file property 'input.image.path': '%s'", filePath)
	err := NewImageCustomizerErrorWithCause(ErrInvalidInput, message, originalErr)

	expectedMessage := fmt.Sprintf("invalid config file property 'input.image.path': '%s':\nfile not found", filePath)
	assert.Equal(t, expectedMessage, err.Error())
	assert.True(t, errors.Is(err, ErrInvalidInput))
	assert.True(t, errors.Is(err, originalErr))
}

func TestCommonErrorConstructors(t *testing.T) {
	// Test NewPackageManagementError
	packages := []string{"package1", "package2"}
	originalErr := errors.New("tdnf error")
	
	err := NewPackageManagementError("install", packages, originalErr)
	assert.Equal(t, "failed to install packages ([package1 package2])", err.Message)
	assert.Equal(t, ErrPackageManagement, err.Type)
	assert.Equal(t, originalErr, err.Cause)
	assert.True(t, errors.Is(err, ErrPackageManagement))
	assert.True(t, errors.Is(err, originalErr))

	// Test NewScriptExecutionError
	scriptErr := errors.New("script execution failed")
	err2 := NewScriptExecutionError("test-script.sh", scriptErr)
	assert.Equal(t, "script (test-script.sh) failed", err2.Message)
	assert.Equal(t, ErrScriptExecution, err2.Type)
	assert.Equal(t, scriptErr, err2.Cause)
	assert.True(t, errors.Is(err2, ErrScriptExecution))
	assert.True(t, errors.Is(err2, scriptErr))

	// Test NewFilesystemOperationError
	fsErr := errors.New("permission denied")
	err3 := NewFilesystemOperationError("read file", "/path/to/file", fsErr)
	assert.Equal(t, "failed to read file (/path/to/file)", err3.Message)
	assert.Equal(t, ErrFilesystemOperation, err3.Type)
	assert.Equal(t, fsErr, err3.Cause)
	assert.True(t, errors.Is(err3, ErrFilesystemOperation))
	assert.True(t, errors.Is(err3, fsErr))
}