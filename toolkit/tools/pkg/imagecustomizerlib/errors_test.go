// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewImageCustomizerError(t *testing.T) {
	t.Run("CreateBasicError", func(t *testing.T) {
		err := NewImageCustomizerError("TestModule:TestError", "test error message")
		assert.NotNil(t, err)
		assert.Equal(t, "test error message", err.Error())
	})

	t.Run("TestErrorWithCategory", func(t *testing.T) {
		err := NewImageCustomizerError("Users:SetUidOnExistingUser", "cannot set UID on existing user")
		assert.NotNil(t, err)
		assert.Equal(t, "cannot set UID on existing user", err.Error())
		
		// Test category parsing
		category := GetErrorCategory(err)
		assert.Equal(t, "users", category)
		
		// Test code parsing
		code := GetErrorCode(err)
		assert.Equal(t, "SetUidOnExistingUser", code)
	})
}

func TestImageCustomizerError_Error(t *testing.T) {
	custErr := NewImageCustomizerError("TestModule:TestError", "original error message")
	assert.Equal(t, "original error message", custErr.Error())
}



func TestGetErrorCategory(t *testing.T) {
	t.Run("ErrorWithCategory", func(t *testing.T) {
		categorizedErr := NewImageCustomizerError("ImageConversion:FormatCheck", "test error")

		category := GetErrorCategory(categorizedErr)
		assert.Equal(t, "imageconversion", category)
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")

		category := GetErrorCategory(originalErr)
		assert.Equal(t, "internal-system", category)
	})

	t.Run("WrappedErrorWithCategory", func(t *testing.T) {
		categorizedErr := NewImageCustomizerError("Filesystem_Operation:FileCopy", "test error")
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)

		category := GetErrorCategory(wrappedErr)
		assert.Equal(t, "filesystem-operation", category)
	})

	t.Run("MultipleWrappedErrorWithCategory", func(t *testing.T) {
		categorizedErr := NewImageCustomizerError("Package_Management:Install", "test error")
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
		doubleWrappedErr := fmt.Errorf("double wrapped: %w", wrappedErr)

		category := GetErrorCategory(doubleWrappedErr)
		assert.Equal(t, "package-management", category)
	})

	t.Run("NilError", func(t *testing.T) {
		category := GetErrorCategory(nil)
		assert.Equal(t, "internal-system", category)
	})
}

func TestIsErrorCategory(t *testing.T) {
	t.Run("MatchingCategory", func(t *testing.T) {
		categorizedErr := NewImageCustomizerError("ScriptExecution:Failure", "test error")

		assert.True(t, IsErrorCategory(categorizedErr, "scriptexecution"))
		assert.False(t, IsErrorCategory(categorizedErr, "invalid-input"))
	})

	t.Run("WrappedMatchingCategory", func(t *testing.T) {
		categorizedErr := NewImageCustomizerError("NetworkOperation:Failure", "test error")
		wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)

		assert.True(t, IsErrorCategory(wrappedErr, "networkoperation"))
		assert.False(t, IsErrorCategory(wrappedErr, "invalid-input"))
	})

	t.Run("ErrorWithoutCategory", func(t *testing.T) {
		originalErr := errors.New("test error")

		assert.True(t, IsErrorCategory(originalErr, "internal-system"))
		assert.False(t, IsErrorCategory(originalErr, "invalid-input"))
	})

	t.Run("NilError", func(t *testing.T) {
		assert.True(t, IsErrorCategory(nil, "internal-system"))
		assert.False(t, IsErrorCategory(nil, "invalid-input"))
	})
}

func TestErrorCategoryPreservationThroughWrapping(t *testing.T) {
	// Test that categories are preserved when errors are wrapped multiple times
	categorizedErr := NewImageCustomizerError("PermissionDenied:AccessDenied", "base error")

	// Wrap the error multiple times
	wrapped1 := fmt.Errorf("layer 1: %w", categorizedErr)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped1)
	wrapped3 := fmt.Errorf("layer 3: %w", wrapped2)

	// Category should still be extractable
	category := GetErrorCategory(wrapped3)
	assert.Equal(t, "permissiondenied", category)

	// IsErrorCategory should still work
	assert.True(t, IsErrorCategory(wrapped3, "permissiondenied"))
	assert.False(t, IsErrorCategory(wrapped3, "invalid-input"))

	// Original error should still be in the chain
	assert.True(t, errors.Is(wrapped3, categorizedErr))
}

func TestErrorCategoriesInRealValidation(t *testing.T) {
	// Create a simple test error with new system
	testErr := NewImageCustomizerError("Invalid_Input:RpmSource", "invalid RPM source")
	
	t.Run("ErrorWrapping_PreservesCategory", func(t *testing.T) {
		// Wrap the error
		wrappedErr := fmt.Errorf("configuration validation failed: %w", testErr)

		// Category should still be extractable
		assert.True(t, IsErrorCategory(wrappedErr, "invalid-input"))
		assert.Equal(t, "invalid-input", GetErrorCategory(wrappedErr))

		// Original error should still be in the chain
		assert.True(t, errors.Is(wrappedErr, testErr))
	})
}

func TestBackwardsCompatibility(t *testing.T) {
	// Test that existing error handling code still works
	categorizedErr := NewImageCustomizerError("InvalidInput:OutputFormat", "test error")

	// Standard error methods should work
	assert.Equal(t, "test error", categorizedErr.Error())

	// Wrapped errors should work
	wrappedErr := fmt.Errorf("wrapped: %w", categorizedErr)
	assert.True(t, errors.Is(wrappedErr, categorizedErr))
}

func TestGetErrorCode(t *testing.T) {
	t.Run("ErrorWithCode", func(t *testing.T) {
		custErr := NewImageCustomizerError("ImageConversion:FormatCheck", "test error")

		code := GetErrorCode(custErr)
		assert.Equal(t, "FormatCheck", code)
	})

	t.Run("WrappedErrorWithCode", func(t *testing.T) {
		custErr := NewImageCustomizerError("PackageManagement:Install", "test error")
		wrappedErr := fmt.Errorf("wrapped: %w", custErr)

		code := GetErrorCode(wrappedErr)
		assert.Equal(t, "Install", code)
	})

	t.Run("ErrorWithoutCode", func(t *testing.T) {
		originalErr := errors.New("test error")
		code := GetErrorCode(originalErr)
		assert.Equal(t, "Unset", code)
	})
}

func TestIsErrorCode(t *testing.T) {
	t.Run("MatchingCode", func(t *testing.T) {
		custErr := NewImageCustomizerError("ScriptExecution:RunFailure", "test error")

		assert.True(t, IsErrorCode(custErr, "RunFailure"))
		assert.False(t, IsErrorCode(custErr, "OutputFormat"))
	})

	t.Run("WrappedMatchingCode", func(t *testing.T) {
		custErr := NewImageCustomizerError("ServiceOperation:Enable", "test error")
		wrappedErr := fmt.Errorf("wrapped: %w", custErr)

		assert.True(t, IsErrorCode(wrappedErr, "Enable"))
		assert.False(t, IsErrorCode(wrappedErr, "Disable"))
	})
}
