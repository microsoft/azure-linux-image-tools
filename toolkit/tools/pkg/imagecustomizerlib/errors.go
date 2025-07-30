// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
)

// ImageCustomizerError represents a structured error with a descriptive name
type ImageCustomizerError struct {
	name    string // descriptive name in format "Module:ErrorType"
	message string // user-friendly error message
}

func (e *ImageCustomizerError) Error() string {
	return e.message
}

// Name returns the error name for telemetry purposes
func (e *ImageCustomizerError) Name() string {
	return e.name
}

// NewImageCustomizerError creates a new named ImageCustomizerError
func NewImageCustomizerError(name, message string) *ImageCustomizerError {
	return &ImageCustomizerError{
		name:    name,
		message: message,
	}
}
