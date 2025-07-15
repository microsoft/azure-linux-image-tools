// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
	"fmt"
)

// Global error types for categorization
var (
	ConfigValidationError     = errors.New("config-validation")
	ImageConversionError      = errors.New("image-conversion")
	FilesystemOperationError  = errors.New("filesystem-operation")
	PackageManagementError    = errors.New("package-management")
	ScriptExecutionError      = errors.New("script-execution")
	InternalSystemError       = errors.New("internal-system")
)

// Static error messages as global variables
var (
	ErrInputImageFileRequired        = errors.New("input image file must be specified, either via the command line option '--image-file' or in the config file property 'input.image.path'")
	ErrOutputImageFileRequired       = errors.New("output image file must be specified, either via the command line option '--output-image-file' or in the config file property 'output.image.path'")
	ErrToolMustRunAsRoot             = errors.New("tool should be run as root (e.g. by using sudo)")
	ErrUkiPreviewFeatureRequired     = errors.New("the 'uki' preview feature must be enabled to use 'os.uki'")
	ErrBootLoaderResetRequired       = errors.New("'os.bootloader.reset' must be specified if 'storage.disks' is specified")
	ErrBootLoaderResetUuidsRequired  = errors.New("'os.bootloader.reset' must be specified if 'storage.resetPartitionsUuidsType' is specified")
	ErrOutputImageFormatRequired     = errors.New("output image format must be specified, either via the command line option '--output-image-format' or in the config file property 'output.image.format'")
	ErrCannotCustomizePartitionsIso  = errors.New("cannot customize partitions when the input is an iso")
	ErrRpmSourcesRequiredForPackages = errors.New("have packages to install or update but no RPM sources were specified")
	ErrKdumpBootFilesPreviewRequired = errors.New("preview feature must be enabled to use 'iso.kdumpBootFiles'")
	ErrRootHashParsingFailed         = errors.New("failed to parse root hash from veritysetup output")
)

// ImageCustomizerError struct for dynamic content
type ImageCustomizerError struct {
	Type    error
	Message string
	Cause   error
}

func (e *ImageCustomizerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s:\n%v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ImageCustomizerError) Unwrap() error {
	return e.Cause
}

func (e *ImageCustomizerError) Is(target error) bool {
	return errors.Is(e.Type, target)
}

// Helper functions for creating ImageCustomizerError instances
func NewImageCustomizerError(errorType error, message string) *ImageCustomizerError {
	return &ImageCustomizerError{
		Type:    errorType,
		Message: message,
		Cause:   nil,
	}
}

func NewImageCustomizerErrorWithCause(errorType error, message string, cause error) *ImageCustomizerError {
	return &ImageCustomizerError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}