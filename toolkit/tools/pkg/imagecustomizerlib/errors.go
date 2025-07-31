// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import "errors"

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

// GetDeepestImageCustomizerError returns the deepest ImageCustomizerError in the error chain
func GetDeepestImageCustomizerError(err error) *ImageCustomizerError {
	var deepest *ImageCustomizerError

	findDeepestInError(err, &deepest)

	return deepest
}

// findDeepestInError recursively traverses an error tree (including multi-wrapped errors)
// and updates the deepest pointer when it finds ImageCustomizerError instances
func findDeepestInError(err error, deepest **ImageCustomizerError) {
	if err == nil {
		return
	}

	var namedErr *ImageCustomizerError
	if errors.As(err, &namedErr) {
		*deepest = namedErr
	}

	if multiErr, ok := err.(interface{ Unwrap() []error }); ok {
		// Multiple wrapped errors - traverse all of them
		unwrappedList := multiErr.Unwrap()
		for _, wrappedErr := range unwrappedList {
			findDeepestInError(wrappedErr, deepest)
		}
	} else {
		// Single wrapped error - use traditional unwrapping
		findDeepestInError(errors.Unwrap(err), deepest)
	}
}
