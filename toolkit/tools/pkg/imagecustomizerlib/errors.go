// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
)

// ErrorCategoryType represents the category of error for telemetry
type ErrorCategoryType int

const (
	ErrorCategoryTypeInvalidInput ErrorCategoryType = iota
	ErrorCategoryTypeImageConversion
	ErrorCategoryTypeFilesystemOperation
	ErrorCategoryTypePackageManagement
	ErrorCategoryTypeScriptExecution
	ErrorCategoryTypeInternalSystem
	ErrorCategoryTypeNetworkOperation
	ErrorCategoryTypePermissionDenied
	ErrorCategoryTypeUserGroupOperation
	ErrorCategoryTypeServiceOperation
)

// String returns the string representation of the error category
func (e ErrorCategoryType) String() string {
	switch e {
	case ErrorCategoryTypeInvalidInput:
		return "invalid-input"
	case ErrorCategoryTypeImageConversion:
		return "image-conversion"
	case ErrorCategoryTypeFilesystemOperation:
		return "filesystem-operation"
	case ErrorCategoryTypePackageManagement:
		return "package-management"
	case ErrorCategoryTypeScriptExecution:
		return "script-execution"
	case ErrorCategoryTypeInternalSystem:
		return "internal-system"
	case ErrorCategoryTypeNetworkOperation:
		return "network-operation"
	case ErrorCategoryTypePermissionDenied:
		return "permission-denied"
	case ErrorCategoryTypeUserGroupOperation:
		return "user-group-operation"
	case ErrorCategoryTypeServiceOperation:
		return "service-operation"
	default:
		return "internal"
	}
}

// ErrorCategory wraps an error with a category for telemetry
type ErrorCategory struct {
	Err      error
	Category ErrorCategoryType
}

func (e *ErrorCategory) Error() string {
	return e.Err.Error()
}

func (e *ErrorCategory) Unwrap() error {
	return e.Err
}

// AttachErrorCategory wraps an error with a category
func AttachErrorCategory(category ErrorCategoryType, err error) error {
	if err == nil {
		return nil
	}
	return &ErrorCategory{
		Err:      err,
		Category: category,
	}
}

// GetErrorCategory extracts the error category from any error in the chain
func GetErrorCategory(err error) ErrorCategoryType {
	var categoryErr *ErrorCategory
	if errors.As(err, &categoryErr) {
		return categoryErr.Category
	}
	return ErrorCategoryTypeInternalSystem // default for uncategorized errors
}

// IsErrorCategory checks if an error has a specific category
func IsErrorCategory(err error, category ErrorCategoryType) bool {
	return GetErrorCategory(err) == category
}