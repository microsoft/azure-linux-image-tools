// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewImageCustomizerError_CreateBasicError(t *testing.T) {
	err := NewImageCustomizerError("TestModule:TestError", "test error message")
	assert.NotNil(t, err)
	assert.Equal(t, "test error message", err.Error())
	assert.Equal(t, "TestModule:TestError", err.Name())
}

func TestNewImageCustomizerError_WithNamedError(t *testing.T) {
	err := NewImageCustomizerError("Users:SetUidOnExistingUser", "cannot set UID on existing user")
	assert.NotNil(t, err)
	assert.Equal(t, "cannot set UID on existing user", err.Error())
	assert.Equal(t, "Users:SetUidOnExistingUser", err.Name())
}

func TestImageCustomizerError_Error(t *testing.T) {
	custErr := NewImageCustomizerError("TestModule:TestError", "original error message")
	assert.Equal(t, "original error message", custErr.Error())
}

func TestImageCustomizerError_Name(t *testing.T) {
	custErr := NewImageCustomizerError("TestModule:TestError", "original error message")
	assert.Equal(t, "TestModule:TestError", custErr.Name())
}

func TestImageCustomizerError_ErrorsAs(t *testing.T) {
	originalErr := NewImageCustomizerError("Test:Operation", "test operation failed")
	wrappedErr := fmt.Errorf("wrapper:\n%w", originalErr)

	var customErr *ImageCustomizerError
	assert.True(t, errors.As(wrappedErr, &customErr))
	assert.Equal(t, "Test:Operation", customErr.Name())
	assert.Equal(t, "test operation failed", customErr.Error())
}

func TestImageCustomizerError_ErrorsIs(t *testing.T) {
	originalErr := NewImageCustomizerError("Test:Operation", "test operation failed")
	wrappedErr := fmt.Errorf("wrapper:\n%w", originalErr)

	assert.True(t, errors.Is(wrappedErr, originalErr))
}

func TestGetAllImageCustomizerErrors_SingleError(t *testing.T) {
	singleErr := NewImageCustomizerError("Single:Error", "single error")
	wrappedSingle := fmt.Errorf("wrapper: %w", singleErr)

	errors := GetAllImageCustomizerErrors(wrappedSingle)
	assert.NotNil(t, errors)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Single:Error", errors[0].Name())
	assert.Equal(t, "single error", errors[0].Error())
}

func TestGetAllImageCustomizerErrors_NoImageCustomizerError(t *testing.T) {
	regularErr := fmt.Errorf("regular error")
	wrappedRegular := fmt.Errorf("wrapper: %w", regularErr)

	errors := GetAllImageCustomizerErrors(wrappedRegular)
	assert.Nil(t, errors)
}

func TestGetAllImageCustomizerErrors_MultipleErrors(t *testing.T) {
	// Create a proper chain: outerErr -> middleWrapper -> innerErr
	innerErr := NewImageCustomizerError("Inner:Error", "inner error message")
	middleWrapper := fmt.Errorf("middle wrapper: %w", innerErr)
	outerErr := NewImageCustomizerError("Outer:Error", "outer error message")
	finalWrapper := fmt.Errorf("final wrapper with %w and also %w", outerErr, middleWrapper)

	errors := GetAllImageCustomizerErrors(finalWrapper)
	assert.NotNil(t, errors)
	assert.Len(t, errors, 2)
	// BFS order: Outer:Error comes first (depth 1), then Inner:Error (depth 2)
	assert.Equal(t, "Outer:Error", errors[0].Name())
	assert.Equal(t, "outer error message", errors[0].Error())
	assert.Equal(t, "Inner:Error", errors[1].Name())
	assert.Equal(t, "inner error message", errors[1].Error())
}

func TestGetAllImageCustomizerErrors_DirectError(t *testing.T) {
	directErr := NewImageCustomizerError("Direct:Error", "direct error")

	errors := GetAllImageCustomizerErrors(directErr)
	assert.NotNil(t, errors)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Direct:Error", errors[0].Name())
}
