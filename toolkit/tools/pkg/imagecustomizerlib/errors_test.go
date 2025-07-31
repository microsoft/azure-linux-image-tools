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
		assert.Equal(t, "TestModule:TestError", err.Name())
	})

	t.Run("TestErrorWithName", func(t *testing.T) {
		err := NewImageCustomizerError("Users:SetUidOnExistingUser", "cannot set UID on existing user")
		assert.NotNil(t, err)
		assert.Equal(t, "cannot set UID on existing user", err.Error())
		assert.Equal(t, "Users:SetUidOnExistingUser", err.Name())
	})
}

func TestImageCustomizerError_Error(t *testing.T) {
	custErr := NewImageCustomizerError("TestModule:TestError", "original error message")
	assert.Equal(t, "original error message", custErr.Error())
}

func TestImageCustomizerError_Name(t *testing.T) {
	custErr := NewImageCustomizerError("TestModule:TestError", "original error message")
	assert.Equal(t, "TestModule:TestError", custErr.Name())
}

func TestErrorWrapping(t *testing.T) {
	t.Run("ErrorsAs", func(t *testing.T) {
		originalErr := NewImageCustomizerError("Test:Operation", "test operation failed")
		wrappedErr := fmt.Errorf("wrapper: \n%w", originalErr)

		var customErr *ImageCustomizerError
		assert.True(t, errors.As(wrappedErr, &customErr))
		assert.Equal(t, "Test:Operation", customErr.Name())
		assert.Equal(t, "test operation failed", customErr.Error())
	})

	t.Run("ErrorsIs", func(t *testing.T) {
		originalErr := NewImageCustomizerError("Test:Operation", "test operation failed")
		wrappedErr := fmt.Errorf("wrapper: \n%w", originalErr)

		assert.True(t, errors.Is(wrappedErr, originalErr))
	})
}
