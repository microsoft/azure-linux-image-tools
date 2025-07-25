// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

type ImageCustomizerError struct {
	name    string
	message string
}

func NewImageCustomizerError(name string, message string) *ImageCustomizerError {
	return &ImageCustomizerError{
		name:    name,
		message: message,
	}
}

func (e *ImageCustomizerError) Name() string {
	return e.name
}

func (e *ImageCustomizerError) Error() string {
	return e.message
}
