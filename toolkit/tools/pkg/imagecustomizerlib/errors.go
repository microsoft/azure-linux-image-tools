// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"container/list"
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

func GetAllImageCustomizerErrors(err error) []*ImageCustomizerError {
	if err == nil {
		return nil
	}

	var result []*ImageCustomizerError
	queue := list.New()
	queue.PushBack(err)

	for queue.Len() > 0 {
		current := queue.Remove(queue.Front()).(error)

		if namedErr, ok := current.(*ImageCustomizerError); ok {
			result = append(result, namedErr)
		}

		var wrappedErrors []error
		if multiErr, ok := current.(interface{ Unwrap() []error }); ok {
			wrappedErrors = multiErr.Unwrap()
		} else if wrappedErr := errors.Unwrap(current); wrappedErr != nil {
			wrappedErrors = []error{wrappedErr}
		}

		for _, wrappedErr := range wrappedErrors {
			queue.PushBack(wrappedErr)
		}
	}

	return result
}
