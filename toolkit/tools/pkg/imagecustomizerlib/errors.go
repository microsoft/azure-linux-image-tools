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
	CodeInvalidOutputFormat              ErrorCode = "Invalid_Output_Format_Failure"
	CodeCannotCustomizePartitionsOnIso  ErrorCode = "Partition_Customization_On_Iso_Failure"
	CodeCannotGenerateOutputFormat       ErrorCode = "Output_Format_Generation_Failure"
	CodeInvalidPackageListFile           ErrorCode = "Package_List_File_Validation_Failure"
	CodeInvalidPasswordFile              ErrorCode = "Password_File_Validation_Failure"
	CodeUnknownRpmSourceType             ErrorCode = "Rpm_Source_Type_Recognition_Failure"
	CodeRpmSourceTypeDetection           ErrorCode = "Rpm_Source_Type_Detection_Failure"

	// Image Conversion errors
	CodeImageFormatCheck                 ErrorCode = "Image_Format_Check_Failure"
	CodeQemuImgInfo                      ErrorCode = "Qemu_Img_Info_Failure"

	// Filesystem Operation errors
	CodeHostnameWrite                    ErrorCode = "Hostname_Write_Failure"
	CodeDirectoryCopy                    ErrorCode = "Directory_Copy_Failure"
	CodeFileCopy                         ErrorCode = "File_Copy_Failure"
	CodeOverlayDirectoryCreate           ErrorCode = "Overlay_Directory_Create_Failure"
	CodeOverlayFstabUpdate               ErrorCode = "Overlay_Fstab_Update_Failure"
	CodeConfigDirMount                   ErrorCode = "Config_Dir_Mount_Failure"
	CodeConfigDirUnmount                 ErrorCode = "Config_Dir_Unmount_Failure"

	// Package Management errors
	CodePackageRepoMetadataRefresh       ErrorCode = "Package_Repo_Metadata_Refresh_Failure"
	CodePackageInstall                   ErrorCode = "Package_Install_Failure"
	CodePackageRemove                    ErrorCode = "Package_Remove_Failure"
	CodePackageUpdate                    ErrorCode = "Package_Update_Failure"
	CodePackageCacheClean                ErrorCode = "Package_Cache_Clean_Failure"
	CodePackageClean                     ErrorCode = "Package_Clean_Failure"
	CodePackageUpgrade                   ErrorCode = "Package_Upgrade_Failure"

	// Script Execution errors
	CodeScriptExecution                  ErrorCode = "Script_Execution_Failure"

	// Service Operation errors
	CodeServiceEnable                    ErrorCode = "Service_Enable_Failure"
	CodeServiceDisable                   ErrorCode = "Service_Disable_Failure"

	// User/Group Operation errors
	CodeGroupGidSet                      ErrorCode = "Group_Gid_Set_Failure"
	CodeUserUidSet                       ErrorCode = "User_Uid_Set_Failure"
	CodeUserHomeDirSet                   ErrorCode = "User_Home_Dir_Set_Failure"
	CodeGroupExists                      ErrorCode = "Group_Exists_Check_Failure"
	CodeGroupAdd                         ErrorCode = "Group_Add_Failure"
	CodeUserExists                       ErrorCode = "User_Exists_Check_Failure"
	CodeUserAdd                          ErrorCode = "User_Add_Failure"
	CodeUserUpdate                       ErrorCode = "User_Update_Failure"
	CodePasswordRead                     ErrorCode = "Password_Read_Failure"

	// Miscellaneous errors for other operations
	CodeOverlayCheck                     ErrorCode = "Overlay_Check_Failure"
	CodeRpmSourceMount                   ErrorCode = "Rpm_Source_Mount_Failure"
	CodeInternalSystem                   ErrorCode = "Internal_System_Failure"
	CodeNetworkOperation                 ErrorCode = "Network_Operation_Failure"
	CodePermissionDenied                 ErrorCode = "Permission_Denied_Failure"
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

// Wrap wraps an error with additional context while preserving the category and code
func (e *ImageCustomizerError) Wrap(msg string) error {
	if e == nil {
		return nil
	}
	return &ImageCustomizerError{
		Category: e.Category,
		Code:     e.Code,
		Err:      errors.New(msg + ": " + e.Err.Error()),
	}
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