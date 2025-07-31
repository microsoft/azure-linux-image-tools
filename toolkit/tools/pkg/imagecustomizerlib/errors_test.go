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

func TestGetDeepestImageCustomizerError(t *testing.T) {
	t.Run("SingleImageCustomizerError", func(t *testing.T) {
		singleErr := NewImageCustomizerError("Single:Error", "single error")
		wrappedSingle := fmt.Errorf("wrapper: %w", singleErr)

		deepest := GetDeepestImageCustomizerError(wrappedSingle)
		assert.NotNil(t, deepest)
		assert.Equal(t, "Single:Error", deepest.Name())
		assert.Equal(t, "single error", deepest.Error())
	})

	t.Run("NoImageCustomizerErrorInChain", func(t *testing.T) {
		regularErr := fmt.Errorf("regular error")
		wrappedRegular := fmt.Errorf("wrapper: %w", regularErr)

		deepest := GetDeepestImageCustomizerError(wrappedRegular)
		assert.Nil(t, deepest)
	})

	t.Run("MultipleImageCustomizerErrors_ReturnsDeepest", func(t *testing.T) {
		// Create a proper chain: outerErr -> middleWrapper -> innerErr
		innerErr := NewImageCustomizerError("Inner:Error", "inner error message")
		middleWrapper := fmt.Errorf("middle wrapper: %w", innerErr)
		outerErr := NewImageCustomizerError("Outer:Error", "outer error message")
		finalWrapper := fmt.Errorf("final wrapper with %w and also %w", outerErr, middleWrapper)

		// The deepest should be the inner error (it's furthest down the chain)
		deepest := GetDeepestImageCustomizerError(finalWrapper)
		assert.NotNil(t, deepest)
		assert.Equal(t, "Inner:Error", deepest.Name())
		assert.Equal(t, "inner error message", deepest.Error())
	})

	t.Run("DirectImageCustomizerError", func(t *testing.T) {
		directErr := NewImageCustomizerError("Direct:Error", "direct error")

		deepest := GetDeepestImageCustomizerError(directErr)
		assert.NotNil(t, deepest)
		assert.Equal(t, "Direct:Error", deepest.Name())
	})
}
