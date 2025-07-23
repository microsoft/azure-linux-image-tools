// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{
			name:     "InvalidInput",
			category: CategoryInvalidInput,
			expected: "Invalid_Input",
		},
		{
			name:     "ImageConversion",
			category: CategoryImageConversion,
			expected: "Image_Conversion",
		},
		{
			name:     "FilesystemOperation",
			category: CategoryFilesystemOperation,
			expected: "Filesystem_Operation",
		},
		{
			name:     "PackageManagement",
			category: CategoryPackageManagement,
			expected: "Package_Management",
		},
		{
			name:     "ScriptExecution",
			category: CategoryScriptExecution,
			expected: "Script_Execution",
		},
		{
			name:     "InternalSystem",
			category: CategoryInternalSystem,
			expected: "Internal_System",
		},
		{
			name:     "NetworkOperation",
			category: CategoryNetworkOperation,
			expected: "Network_Operation",
		},
		{
			name:     "PermissionDenied",
			category: CategoryPermissionDenied,
			expected: "Permission_Denied",
		},
		{
			name:     "UserGroupOperation",
			category: CategoryUserGroupOperation,
			expected: "User_Group_Operation",
		},
		{
			name:     "ServiceOperation",
			category: CategoryServiceOperation,
			expected: "Service_Operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.category.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewImageCustomizerError(t *testing.T) {
	t.Run("CreateWithNilError", func(t *testing.T) {
		err := NewImageCustomizerError(CategoryInvalidInput, CodeInvalidOutputFormat, nil)
		assert.Nil(t, err)
	})

	t.Run("CreateWithValidError", func(t *testing.T) {
		originalErr := errors.New("test error")
		err := NewImageCustomizerError(CategoryInvalidInput, CodeInvalidOutputFormat, originalErr)
		
		assert.NotNil(t, err)
		assert.Equal(t, "test error", err.Error())
		
		// Verify it's wrapped correctly
		assert.True(t, errors.Is(err, originalErr))
	})
}

func TestImageCustomizerError_Error(t *testing.T) {
	originalErr := errors.New("original error message")
	custErr := &ImageCustomizerError{
		Err:      originalErr,
		Category: CategoryInvalidInput,
		Code:     CodeInvalidOutputFormat,
	}

	assert.Equal(t, "original error message", custErr.Error())
}

func TestImageCustomizerError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	custErr := &ImageCustomizerError{
		Err:      originalErr,
		Category: CategoryInvalidInput,
		Code:     CodeInvalidOutputFormat,
	}

	unwrapped := custErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestGetErrorCategory(t *testing.T) {
	t.Run("ErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := NewImageCustomizerError(CategoryImageConversion, CodeImageFormatCheck, originalErr)
		
		category := GetErrorCategory(categorizedErr)
		assert.Equal(t, CategoryImageConversion, category)
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		
		category := GetErrorCategory(originalErr)
		assert.Equal(t, CategoryInternalSystem, category)
	})

	t.Run("WrappedErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := NewImageCustomizerError(CategoryFilesystemOperation, CodeFileCopy, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		
		category := GetErrorCategory(wrappedErr)
		assert.Equal(t, CategoryFilesystemOperation, category)
	})

	t.Run("MultipleWrappedErrorWithCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := NewImageCustomizerError(CategoryPackageManagement, CodePackageInstall, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		doubleWrappedErr := fmt.Errorf("double wrapped: %w", wrappedErr)
		
		category := GetErrorCategory(doubleWrappedErr)
		assert.Equal(t, CategoryPackageManagement, category)
	})

	t.Run("NilError", func(t *testing.T) {
		category := GetErrorCategory(nil)
		assert.Equal(t, CategoryInternalSystem, category)
	})
}

func TestIsErrorCategory(t *testing.T) {
	t.Run("MatchingCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := NewImageCustomizerError(CategoryScriptExecution, CodeScriptExecution, originalErr)
		
		assert.True(t, IsErrorCategory(categorizedErr, CategoryScriptExecution))
		assert.False(t, IsErrorCategory(categorizedErr, CategoryInvalidInput))
	})

	t.Run("WrappedMatchingCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		categorizedErr := NewImageCustomizerError(CategoryNetworkOperation, CodeNetworkOperation, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		
		assert.True(t, IsErrorCategory(wrappedErr, CategoryNetworkOperation))
		assert.False(t, IsErrorCategory(wrappedErr, CategoryInvalidInput))
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")
		
		assert.True(t, IsErrorCategory(originalErr, CategoryInternalSystem))
		assert.False(t, IsErrorCategory(originalErr, CategoryInvalidInput))
	})

	t.Run("NilError", func(t *testing.T) {
		assert.True(t, IsErrorCategory(nil, CategoryInternalSystem))
		assert.False(t, IsErrorCategory(nil, CategoryInvalidInput))
	})
}

func TestErrorCategoryPreservationThroughWrapping(t *testing.T) {
	// Test that categories are preserved when errors are wrapped multiple times
	originalErr := errors.New("base error")
	categorizedErr := NewImageCustomizerError(CategoryPermissionDenied, CodePermissionDenied, originalErr)
	
	// Wrap the error multiple times
	wrapped1 := fmt.Errorf("layer 1: %w", categorizedErr)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped1)
	wrapped3 := fmt.Errorf("layer 3: %w", wrapped2)
	
	// Category should still be extractable
	category := GetErrorCategory(wrapped3)
	assert.Equal(t, CategoryPermissionDenied, category)
	
	// IsErrorCategory should still work
	assert.True(t, IsErrorCategory(wrapped3, CategoryPermissionDenied))
	assert.False(t, IsErrorCategory(wrapped3, CategoryInvalidInput))
	
	// Original error should still be in the chain
	assert.True(t, errors.Is(wrapped3, originalErr))
}

func TestTelemetryIntegration(t *testing.T) {
	// Test showing how error categories can be used for telemetry
	testCases := []struct {
		name           string
		errorGenerator func() error
		expectedCategory ErrorCategory
	}{
		{
			name: "InvalidInput",
			errorGenerator: func() error {
				return NewImageCustomizerError(CategoryInvalidInput, CodeInvalidOutputFormat, errors.New("invalid input"))
			},
			expectedCategory: CategoryInvalidInput,
		},
		{
			name: "ImageConversion",
			errorGenerator: func() error {
				return NewImageCustomizerError(CategoryImageConversion, CodeImageFormatCheck, errors.New("conversion failed"))
			},
			expectedCategory: CategoryImageConversion,
		},
		{
			name: "ScriptExecution",
			errorGenerator: func() error {
				return NewImageCustomizerError(CategoryScriptExecution, CodeScriptExecution, errors.New("script failed"))
			},
			expectedCategory: CategoryScriptExecution,
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

func TestErrorCategoriesInRealValidation(t *testing.T) {
	// Test that error categories work with real validation functions
	t.Run("ValidateRpmSources_InvalidInput", func(t *testing.T) {
		// Test with invalid RPM source
		err := ValidateRpmSources([]string{"/non/existent/path.invalidext"})
		assert.NotNil(t, err)
		
		// Should have InvalidInput category
		assert.True(t, IsErrorCategory(err, CategoryInvalidInput))
		assert.Equal(t, CategoryInvalidInput, GetErrorCategory(err))
	})
	
	t.Run("ValidateRpmSources_ValidInput", func(t *testing.T) {
		// Test with valid (empty) RPM sources
		err := ValidateRpmSources([]string{})
		assert.Nil(t, err)
	})
	
	t.Run("ErrorWrapping_PreservesCategory", func(t *testing.T) {
		// Test that when errors are wrapped, categories are preserved
		err := ValidateRpmSources([]string{"/non/existent/path.invalidext"})
		assert.NotNil(t, err)
		
		// Wrap the error
		wrappedErr := fmt.Errorf("configuration validation failed: %w", err)
		
		// Category should still be extractable
		assert.True(t, IsErrorCategory(wrappedErr, CategoryInvalidInput))
		assert.Equal(t, CategoryInvalidInput, GetErrorCategory(wrappedErr))
		
		// Original error should still be in the chain
		assert.True(t, errors.Is(wrappedErr, err))
	})
}

func TestBackwardsCompatibility(t *testing.T) {
	// Test that existing error handling code still works
	originalErr := errors.New("test error")
	categorizedErr := NewImageCustomizerError(CategoryInvalidInput, CodeInvalidOutputFormat, originalErr)
	
	// Standard error methods should work
	assert.Equal(t, "test error", categorizedErr.Error())
	assert.True(t, errors.Is(categorizedErr, originalErr))
	
	// Wrapped errors should work
	wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.True(t, errors.Is(wrappedErr, categorizedErr))
}

func TestGetErrorCode(t *testing.T) {
	t.Run("ErrorWithCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		custErr := NewImageCustomizerError(CategoryImageConversion, CodeImageFormatCheck, originalErr)
		
		code := GetErrorCode(custErr)
		assert.Equal(t, CodeImageFormatCheck, code)
	})
	
	t.Run("WrappedErrorWithCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		custErr := NewImageCustomizerError(CategoryPackageManagement, CodePackageInstall, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", custErr)
		
		code := GetErrorCode(wrappedErr)
		assert.Equal(t, CodePackageInstall, code)
	})
	
	t.Run("ErrorWithoutCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		code := GetErrorCode(originalErr)
		assert.Equal(t, ErrorCode(""), code)
	})
}

func TestIsErrorCode(t *testing.T) {
	t.Run("MatchingCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		custErr := NewImageCustomizerError(CategoryScriptExecution, CodeScriptExecution, originalErr)
		
		assert.True(t, IsErrorCode(custErr, CodeScriptExecution))
		assert.False(t, IsErrorCode(custErr, CodeInvalidOutputFormat))
	})
	
	t.Run("WrappedMatchingCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		custErr := NewImageCustomizerError(CategoryServiceOperation, CodeServiceEnable, originalErr)
		wrappedErr := fmt.Errorf("wrapped: %w", custErr)
		
		assert.True(t, IsErrorCode(wrappedErr, CodeServiceEnable))
		assert.False(t, IsErrorCode(wrappedErr, CodeServiceDisable))
	})
}