// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"errors"
)

// ErrorCategory represents the category of error for telemetry
type ErrorCategory string

const (
	CategoryInvalidInput        ErrorCategory = "Invalid_Input"
	CategoryImageConversion     ErrorCategory = "Image_Conversion"
	CategoryFilesystemOperation ErrorCategory = "Filesystem_Operation"
	CategoryPackageManagement   ErrorCategory = "Package_Management"
	CategoryScriptExecution     ErrorCategory = "Script_Execution"
	CategoryInternalSystem      ErrorCategory = "Internal_System"
	CategoryNetworkOperation    ErrorCategory = "Network_Operation"
	CategoryPermissionDenied    ErrorCategory = "Permission_Denied"
	CategoryUserGroupOperation  ErrorCategory = "User_Group_Operation"
	CategoryServiceOperation    ErrorCategory = "Service_Operation"
)

// String returns the string representation of the error category
func (e ErrorCategory) String() string {
	return string(e)
}

// ErrorCode is a fine-grained, unique identifier for each specific failure
type ErrorCode string

const (
	// Invalid Input errors
	CodeInvalidOutputFormat              ErrorCode = "Invalid_Output_Format"
	CodeCannotCustomizePartitionsOnIso  ErrorCode = "Cannot_Customize_Partitions_On_Iso"
	CodeCannotGenerateOutputFormat       ErrorCode = "Cannot_Generate_Output_Format"
	CodeInvalidPackageListFile           ErrorCode = "Invalid_Package_List_File"
	CodeInvalidPasswordFile              ErrorCode = "Invalid_Password_File"
	CodeUnknownRpmSourceType             ErrorCode = "Unknown_Rpm_Source_Type"
	CodeFailedToGetRpmSourceType         ErrorCode = "Failed_To_Get_Rpm_Source_Type"

	// Image Conversion errors
	CodeFailedToCheckImageFormat         ErrorCode = "Failed_To_Check_Image_Format"
	CodeFailedToQemuImgInfo              ErrorCode = "Failed_To_Qemu_Img_Info"

	// Filesystem Operation errors
	CodeFailedToWriteHostnameFile        ErrorCode = "Failed_To_Write_Hostname_File"
	CodeFailedToCopyDirectory            ErrorCode = "Failed_To_Copy_Directory"
	CodeFailedToCreateOverlayDirectories ErrorCode = "Failed_To_Create_Overlay_Directories"
	CodeFailedToUpdateFstabForOverlays   ErrorCode = "Failed_To_Update_Fstab_For_Overlays"
	CodeFailedToMountConfigDir           ErrorCode = "Failed_To_Mount_Config_Dir"
	CodeFailedToUnmountConfigDir         ErrorCode = "Failed_To_Unmount_Config_Dir"

	// Package Management errors
	CodeFailedToRefreshTdnfRepoMetadata  ErrorCode = "Failed_To_Refresh_Tdnf_Repo_Metadata"
	CodeFailedToInstallPackages          ErrorCode = "Failed_To_Install_Packages"
	CodeFailedToRemovePackages           ErrorCode = "Failed_To_Remove_Packages"
	CodeFailedToUpdatePackages           ErrorCode = "Failed_To_Update_Packages"
	CodeFailedToCleanTdnfCache           ErrorCode = "Failed_To_Clean_Tdnf_Cache"

	// Script Execution errors
	CodeScriptExecutionFailed            ErrorCode = "Script_Execution_Failed"

	// Service Operation errors
	CodeFailedToEnableService            ErrorCode = "Failed_To_Enable_Service"
	CodeFailedToDisableService           ErrorCode = "Failed_To_Disable_Service"

	// User/Group Operation errors
	CodeCannotSetGidOnExistingGroup      ErrorCode = "Cannot_Set_Gid_On_Existing_Group"
	CodeCannotSetUidOnExistingUser       ErrorCode = "Cannot_Set_Uid_On_Existing_User"
	CodeCannotSetHomeDirOnExistingUser   ErrorCode = "Cannot_Set_Home_Dir_On_Existing_User"
)

// ImageCustomizerError represents a structured error with category and unique code
type ImageCustomizerError struct {
	Category ErrorCategory // non-unique grouping
	Code     ErrorCode     // unique per leaf error
	Err      error         // underlying cause
}

func (e *ImageCustomizerError) Error() string {
	return e.Err.Error()
}

func (e *ImageCustomizerError) Unwrap() error {
	return e.Err
}

// NewImageCustomizerError creates a new ImageCustomizerError
func NewImageCustomizerError(category ErrorCategory, code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &ImageCustomizerError{
		Category: category,
		Code:     code,
		Err:      err,
	}
}

// GetErrorCategory extracts the error category from any error in the chain
func GetErrorCategory(err error) ErrorCategory {
	var custErr *ImageCustomizerError
	if errors.As(err, &custErr) {
		return custErr.Category
	}
	return CategoryInternalSystem // default for uncategorized errors
}

// GetErrorCode extracts the error code from any error in the chain
func GetErrorCode(err error) ErrorCode {
	var custErr *ImageCustomizerError
	if errors.As(err, &custErr) {
		return custErr.Code
	}
	return ""
}

// IsErrorCategory checks if an error has a specific category
func IsErrorCategory(err error, category ErrorCategory) bool {
	return GetErrorCategory(err) == category
}

// IsErrorCode checks if an error has a specific code
func IsErrorCode(err error, code ErrorCode) bool {
	return GetErrorCode(err) == code
}

// Legacy functions for backward compatibility during transition
// These maintain the old function signatures but use the new system

// AttachErrorCategory wraps an error with a category (legacy function)
// Deprecated: Use NewImageCustomizerError with appropriate ErrorCode instead
func AttachErrorCategory(category ErrorCategory, err error) error {
	if err == nil {
		return nil
	}
	// Map to appropriate error code based on category and error message
	code := inferErrorCodeFromError(err)
	return NewImageCustomizerError(category, code, err)
}

// inferErrorCodeFromError attempts to infer an appropriate error code from the error message
func inferErrorCodeFromError(err error) ErrorCode {
	if err == nil {
		return ""
	}
	
	errMsg := err.Error()
	
	// Try to match common error patterns to appropriate codes
	switch {
	case contains(errMsg, "cannot customize partitions when the input is an iso"):
		return CodeCannotCustomizePartitionsOnIso
	case contains(errMsg, "cannot generate output format"):
		return CodeCannotGenerateOutputFormat
	case contains(errMsg, "invalid output image format"):
		return CodeInvalidOutputFormat
	case contains(errMsg, "failed to read package list file"):
		return CodeInvalidPackageListFile
	case contains(errMsg, "failed to read password file"):
		return CodeInvalidPasswordFile
	case contains(errMsg, "unknown RPM source type"):
		return CodeUnknownRpmSourceType
	case contains(errMsg, "failed to get type of RPM source"):
		return CodeFailedToGetRpmSourceType
	case contains(errMsg, "failed to check image file's disk format"):
		return CodeFailedToCheckImageFormat
	case contains(errMsg, "failed to qemu-img info JSON"):
		return CodeFailedToQemuImgInfo
	case contains(errMsg, "failed to write hostname file"):
		return CodeFailedToWriteHostnameFile
	case contains(errMsg, "failed to copy directory"):
		return CodeFailedToCopyDirectory
	case contains(errMsg, "failed to create overlay directories"):
		return CodeFailedToCreateOverlayDirectories
	case contains(errMsg, "failed to update fstab file for overlays"):
		return CodeFailedToUpdateFstabForOverlays
	case contains(errMsg, "failed to refresh tdnf repo metadata"):
		return CodeFailedToRefreshTdnfRepoMetadata
	case contains(errMsg, "failed to install packages") || contains(errMsg, "failed to add packages"):
		return CodeFailedToInstallPackages
	case contains(errMsg, "failed to remove packages"):
		return CodeFailedToRemovePackages
	case contains(errMsg, "failed to update packages"):
		return CodeFailedToUpdatePackages
	case contains(errMsg, "failed to clean tdnf cache"):
		return CodeFailedToCleanTdnfCache
	case contains(errMsg, "script") && contains(errMsg, "failed"):
		return CodeScriptExecutionFailed
	case contains(errMsg, "failed to enable service"):
		return CodeFailedToEnableService
	case contains(errMsg, "failed to disable service"):
		return CodeFailedToDisableService
	case contains(errMsg, "cannot set GID") && contains(errMsg, "on a group") && contains(errMsg, "that already exists"):
		return CodeCannotSetGidOnExistingGroup
	case contains(errMsg, "cannot set UID") && contains(errMsg, "on a user") && contains(errMsg, "that already exists"):
		return CodeCannotSetUidOnExistingUser
	case contains(errMsg, "cannot set home directory") && contains(errMsg, "on a user") && contains(errMsg, "that already exists"):
		return CodeCannotSetHomeDirOnExistingUser
	default:
		return ""
	}
}

// contains is a simple substring check helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
				containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}