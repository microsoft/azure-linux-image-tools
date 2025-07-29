// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"strings"
)

// ImageCustomizerError represents a structured error with a descriptive name
type ImageCustomizerError struct {
	Name    string // descriptive name in format "Module:ErrorType"
	Message string // user-friendly error message
}

func (e *ImageCustomizerError) Error() string {
	return e.Message
}

// NewImageCustomizerError creates a new named ImageCustomizerError
func NewImageCustomizerError(name, message string) *ImageCustomizerError {
	return &ImageCustomizerError{
		Name:    name,
		Message: message,
	}
}

// GetErrorCategory extracts the error category from any error in the chain
// For backwards compatibility with telemetry
func GetErrorCategory(err error) string {
	if err == nil {
		return "internal-system"
	}
	var custErr *ImageCustomizerError
	if errors.As(err, &custErr) {
		// Parse category from name (e.g., "Users:SetUidOnExistingUser" -> "users")
		parts := strings.Split(custErr.Name, ":")
		if len(parts) > 0 {
			category := strings.ToLower(parts[0])
			return strings.ReplaceAll(category, "_", "-")
		}
	}
	return "internal-system" // default for uncategorized errors
}

// GetErrorCode extracts the error code from any error in the chain
// For backwards compatibility with telemetry
func GetErrorCode(err error) string {
	if err == nil {
		return "Unset"
	}
	var custErr *ImageCustomizerError
	if errors.As(err, &custErr) {
		// Parse code from name (e.g., "Users:SetUidOnExistingUser" -> "SetUidOnExistingUser")
		parts := strings.Split(custErr.Name, ":")
		if len(parts) > 1 {
			return parts[1]
		}
		// If no colon, use the full name as code
		return custErr.Name
	}
	return "Unset"
}

// IsErrorCategory checks if an error has a specific category
func IsErrorCategory(err error, category string) bool {
	return GetErrorCategory(err) == category
}

// IsErrorCode checks if an error has a specific code
func IsErrorCode(err error, code string) bool {
	return GetErrorCode(err) == code
}
