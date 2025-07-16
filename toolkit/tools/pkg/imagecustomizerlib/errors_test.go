// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCategoryType_String(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategoryType
		expected string
	}{
		{
			name:     "InvalidInput",
			category: ErrorCategoryTypeInvalidInput,
			expected: "invalid-input",
		},
		{
			name:     "ImageConversion",
			category: ErrorCategoryTypeImageConversion,
			expected: "image-conversion",
		},
		{
			name:     "FilesystemOperation",
			category: ErrorCategoryTypeFilesystemOperation,
			expected: "filesystem-operation",
		},
		{
			name:     "PackageManagement",
			category: ErrorCategoryTypePackageManagement,
			expected: "package-management",
		},
		{
			name:     "ScriptExecution",
			category: ErrorCategoryTypeScriptExecution,
			expected: "script-execution",
		},
		{
			name:     "InternalSystem",
			category: ErrorCategoryTypeInternalSystem,
			expected: "internal-system",
		},
		{
			name:     "NetworkOperation",
			category: ErrorCategoryTypeNetworkOperation,
			expected: "network-operation",
		},
		{
			name:     "PermissionDenied",
			category: ErrorCategoryTypePermissionDenied,
			expected: "permission-denied",
		},
		{
			name:     "Unknown",
			category: ErrorCategoryType(999),
			expected: "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.category.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAttachErrorCategory(t *testing.T) {
	t.Run("AttachToNilError", func(t *testing.T) {
		err := AttachErrorCategory(ErrorCategoryTypeInvalidInput, nil)
		assert.Nil(t, err)
	})

	t.Run("AttachToValidError", func(t *testing.T) {
		originalErr := errors.New("test error")
		err := AttachErrorCategory(ErrorCategoryTypeInvalidInput, originalErr)
		
		assert.NotNil(t, err)
		assert.Equal(t, "test error", err.Error())
		
		// Verify it's wrapped correctly
		assert.True(t, errors.Is(err, originalErr))
	})
}

func TestErrorCategory_Error(t *testing.T) {
	originalErr := errors.New("original error message")
	categoryErr := &ErrorCategory{
		Err:      originalErr,
		Category: ErrorCategoryTypeInvalidInput,
	}

	assert.Equal(t, "original error message", categoryErr.Error())
}

func TestErrorCategory_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	categoryErr := &ErrorCategory{
		Err:      originalErr,
		Category: ErrorCategoryTypeInvalidInput,
	}

	unwrapped := categoryErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestGetErrorCategory(t *testing.T) {
	t.Run("ErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := AttachErrorCategory(ErrorCategoryTypeImageConversion, originalErr)
		
		category := GetErrorCategory(categorizedErr)
		assert.Equal(t, ErrorCategoryTypeImageConversion, category)
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		
		category := GetErrorCategory(originalErr)
		assert.Equal(t, ErrorCategoryTypeInternalSystem, category)
	})

	t.Run("WrappedErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := AttachErrorCategory(ErrorCategoryTypeFilesystemOperation, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		
		category := GetErrorCategory(wrappedErr)
		assert.Equal(t, ErrorCategoryTypeFilesystemOperation, category)
	})

	t.Run("MultipleWrappedErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := AttachErrorCategory(ErrorCategoryTypePackageManagement, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		doubleWrappedErr := fmt.Errorf("double wrapped: %w", wrappedErr)
		
		category := GetErrorCategory(doubleWrappedErr)
		assert.Equal(t, ErrorCategoryTypePackageManagement, category)
	})

	t.Run("NilError", func(t *testing.T) {
		category := GetErrorCategory(nil)
		assert.Equal(t, ErrorCategoryTypeInternalSystem, category)
	})
}

func TestIsErrorCategory(t *testing.T) {
	t.Run("MatchingCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := AttachErrorCategory(ErrorCategoryTypeScriptExecution, originalErr)
		
		assert.True(t, IsErrorCategory(categorizedErr, ErrorCategoryTypeScriptExecution))
		assert.False(t, IsErrorCategory(categorizedErr, ErrorCategoryTypeInvalidInput))
	})

	t.Run("WrappedMatchingCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := AttachErrorCategory(ErrorCategoryTypeNetworkOperation, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		
		assert.True(t, IsErrorCategory(wrappedErr, ErrorCategoryTypeNetworkOperation))
		assert.False(t, IsErrorCategory(wrappedErr, ErrorCategoryTypeInvalidInput))
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		
		assert.True(t, IsErrorCategory(originalErr, ErrorCategoryTypeInternalSystem))
		assert.False(t, IsErrorCategory(originalErr, ErrorCategoryTypeInvalidInput))
	})

	t.Run("NilError", func(t *testing.T) {
		assert.True(t, IsErrorCategory(nil, ErrorCategoryTypeInternalSystem))
		assert.False(t, IsErrorCategory(nil, ErrorCategoryTypeInvalidInput))
	})
}

func TestErrorCategoryPreservationThroughWrapping(t *testing.T) {
	// Test that categories are preserved when errors are wrapped multiple times
	originalErr := errors.New("base error")
	categorizedErr := AttachErrorCategory(ErrorCategoryTypePermissionDenied, originalErr)
	
	// Wrap the error multiple times
	wrapped1 := fmt.Errorf("layer 1: %w", categorizedErr)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped1)
	wrapped3 := fmt.Errorf("layer 3: %w", wrapped2)
	
	// Category should still be extractable
	category := GetErrorCategory(wrapped3)
	assert.Equal(t, ErrorCategoryTypePermissionDenied, category)
	
	// IsErrorCategory should still work
	assert.True(t, IsErrorCategory(wrapped3, ErrorCategoryTypePermissionDenied))
	assert.False(t, IsErrorCategory(wrapped3, ErrorCategoryTypeInvalidInput))
	
	// Original error should still be in the chain
	assert.True(t, errors.Is(wrapped3, originalErr))
}

func TestTelemetryIntegration(t *testing.T) {
	// Test showing how error categories can be used for telemetry
	testCases := []struct {
		name           string
		errorGenerator func() error
		expectedCategory ErrorCategoryType
	}{
		{
			name: "InvalidInput",
			errorGenerator: func() error {
				return AttachErrorCategory(ErrorCategoryTypeInvalidInput, errors.New("invalid input"))
			},
			expectedCategory: ErrorCategoryTypeInvalidInput,
		},
		{
			name: "ImageConversion",
			errorGenerator: func() error {
				return AttachErrorCategory(ErrorCategoryTypeImageConversion, errors.New("conversion failed"))
			},
			expectedCategory: ErrorCategoryTypeImageConversion,
		},
		{
			name: "ScriptExecution",
			errorGenerator: func() error {
				return AttachErrorCategory(ErrorCategoryTypeScriptExecution, errors.New("script failed"))
			},
			expectedCategory: ErrorCategoryTypeScriptExecution,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.errorGenerator()
			
			// This is how telemetry would categorize errors
			category := GetErrorCategory(err)
			assert.Equal(t, tc.expectedCategory, category)
			
			// This is how telemetry would get the category string
			categoryString := category.String()
			assert.NotEmpty(t, categoryString)
			
			// Verify the error is still a normal error
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		})
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	// Test that existing error handling code still works
	originalErr := errors.New("test error")
	categorizedErr := AttachErrorCategory(ErrorCategoryTypeInvalidInput, originalErr)
	
	// Standard error methods should work
	assert.Equal(t, "test error", categorizedErr.Error())
	assert.True(t, errors.Is(categorizedErr, originalErr))
	
	// Wrapped errors should work
	wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.True(t, errors.Is(wrappedErr, categorizedErr))
}