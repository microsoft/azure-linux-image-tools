// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
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
	CategoryImageHistory        ErrorCategory = "Image_History"
	CategoryConfigValidation    ErrorCategory = "Config_Validation"
	CategoryFilesystemCheck     ErrorCategory = "Filesystem_Check"
	CategoryPartitionOperation  ErrorCategory = "Partition_Operation"
	CategoryUKIOperation        ErrorCategory = "UKI_Operation"
	CategorySELinuxOperation    ErrorCategory = "SELinux_Operation"
	CategoryBootCustomization   ErrorCategory = "Boot_Customization"
	CategoryArtifactHandling    ErrorCategory = "Artifact_Handling"
)

// ErrorCode is a fine-grained, unique identifier for each specific failure
type ErrorCode string

const (
	// Default/unset error code
	CodeUnset                            ErrorCode = "Unset"
	
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
	
	// Image History errors
	CodeImageHistoryDeepCopy             ErrorCode = "Image_History_Deep_Copy_Failure"
	CodeImageHistoryModify               ErrorCode = "Image_History_Modify_Failure"
	CodeImageHistoryDirectoryCreate      ErrorCode = "Image_History_Directory_Create_Failure"
	CodeImageHistoryRead                 ErrorCode = "Image_History_Read_Failure"
	CodeImageHistoryWrite                ErrorCode = "Image_History_Write_Failure"
	CodeImageHistoryFileCheck            ErrorCode = "Image_History_File_Check_Failure"
	CodeImageHistoryFileRead             ErrorCode = "Image_History_File_Read_Failure"
	CodeImageHistoryUnmarshal            ErrorCode = "Image_History_Unmarshal_Failure"
	CodeImageHistoryMarshal              ErrorCode = "Image_History_Marshal_Failure"
	CodeImageHistoryFileWrite            ErrorCode = "Image_History_File_Write_Failure"
	
	// Config Validation errors
	CodeConfigKdumpBootFiles             ErrorCode = "Config_Kdump_Boot_Files_Failure"
	CodeConfigKernelCommandLine          ErrorCode = "Config_Kernel_Command_Line_Failure"
	CodeConfigBootstrapUrl               ErrorCode = "Config_Bootstrap_Url_Failure"
	CodeConfigIsoField                   ErrorCode = "Config_Iso_Field_Failure"
	CodeConfigPxeField                   ErrorCode = "Config_Pxe_Field_Failure"
	CodeConfigOsField                    ErrorCode = "Config_Os_Field_Failure"
	CodeConfigDirectoryCreate            ErrorCode = "Config_Directory_Create_Failure"
	CodeConfigFilePersist                ErrorCode = "Config_File_Persist_Failure"
	CodeConfigFileExists                 ErrorCode = "Config_File_Exists_Failure"
	CodeConfigFileLoad                   ErrorCode = "Config_File_Load_Failure"
	
	// Filesystem Check errors
	CodeFilesystemE2fsckCheck            ErrorCode = "Filesystem_E2fsck_Check_Failure"
	CodeFilesystemXfsRepairCheck         ErrorCode = "Filesystem_Xfs_Repair_Check_Failure"
	CodeFilesystemFsckCheck              ErrorCode = "Filesystem_Fsck_Check_Failure"
	
	// Release File errors
	CodeReleaseFileWrite                 ErrorCode = "Release_File_Write_Failure"
	
	// Filesystem Shrink errors
	CodeFilesystemSectorSizeGet          ErrorCode = "Filesystem_Sector_Size_Get_Failure"
	CodeFilesystemShrink                 ErrorCode = "Filesystem_Shrink_Failure"
	CodeFilesystemE2fsckResize           ErrorCode = "Filesystem_E2fsck_Resize_Failure"
	CodeFilesystemResize2fs              ErrorCode = "Filesystem_Resize2fs_Failure"
	
	// Partition Operation errors
	CodePartitionExtractAbsolutePath     ErrorCode = "Partition_Extract_Absolute_Path_Failure"
	CodePartitionExtractIntegrityCheck   ErrorCode = "Partition_Extract_Integrity_Check_Failure"
	CodePartitionExtractStatFile         ErrorCode = "Partition_Extract_Stat_File_Failure"
	CodePartitionExtractUnsupportedFormat ErrorCode = "Partition_Extract_Unsupported_Format_Failure"
	CodePartitionExtractMetadataConstruct ErrorCode = "Partition_Extract_Metadata_Construct_Failure"
	CodePartitionExtractRemoveRawFile    ErrorCode = "Partition_Extract_Remove_Raw_File_Failure"
	CodePartitionExtractRemoveTempFile   ErrorCode = "Partition_Extract_Remove_Temp_File_Failure"
	CodePartitionExtractCopyBlockDevice  ErrorCode = "Partition_Extract_Copy_Block_Device_Failure"
	CodePartitionExtractCompress         ErrorCode = "Partition_Extract_Compress_Failure"
	CodePartitionExtractOpenFile         ErrorCode = "Partition_Extract_Open_File_Failure"
	
	// UKI Operation errors
	CodeUKIPackageDependencyValidation   ErrorCode = "UKI_Package_Dependency_Validation_Failure"
	CodeUKIDirectoryCreate               ErrorCode = "UKI_Directory_Create_Failure"
	CodeUKIShimFileCopy                  ErrorCode = "UKI_Shim_File_Copy_Failure"
	CodeUKISystemdBootInstall            ErrorCode = "UKI_Systemd_Boot_Install_Failure"
	CodeUKIRandomSeedRemove              ErrorCode = "UKI_Random_Seed_Remove_Failure"
	CodeUKIKernelInitramfsMap            ErrorCode = "UKI_Kernel_Initramfs_Map_Failure"
	CodeUKIFileCopy                      ErrorCode = "UKI_File_Copy_Failure"
	CodeUKIKernelCmdlineExtract          ErrorCode = "UKI_Kernel_Cmdline_Extract_Failure"
	CodeUKICmdlineFileWrite              ErrorCode = "UKI_Cmdline_File_Write_Failure"
	
	// Dracut Operation errors
	CodeAddDracutDriver                  ErrorCode = "Add_Dracut_Driver_Failure"
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
	return CodeUnset
}

// IsErrorCategory checks if an error has a specific category
func IsErrorCategory(err error, category ErrorCategory) bool {
	return GetErrorCategory(err) == category
}

// IsErrorCode checks if an error has a specific code
func IsErrorCode(err error, code ErrorCode) bool {
	return GetErrorCode(err) == code
}

// GetErrorJSON returns a JSON representation of the error category and code for telemetry
// This excludes the actual error message to avoid PII in telemetry data
func GetErrorJSON(err error) string {
	category := GetErrorCategory(err)
	code := GetErrorCode(err)
	
	errorDetails := map[string]string{
		"category": string(category),
	}
	if code != CodeUnset {
		errorDetails["code"] = string(code)
	}
	
	// Marshal to JSON, ignoring any errors since this is for telemetry
	errorDetailsJSON, _ := json.Marshal(errorDetails)
	return string(errorDetailsJSON)
}
