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
	CategoryImageHistory        ErrorCategory = "Image_History"
	CategoryConfigValidation    ErrorCategory = "Config_Validation"
	CategoryFilesystemCheck     ErrorCategory = "Filesystem_Check"
	CategoryPartitionOperation  ErrorCategory = "Partition_Operation"
	CategoryUKIOperation        ErrorCategory = "UKI_Operation"
	CategorySELinuxOperation    ErrorCategory = "SELinux_Operation"
	CategoryBootCustomization   ErrorCategory = "Boot_Customization"
	CategoryArtifactHandling    ErrorCategory = "Artifact_Handling"
	CategoryDracutOperation     ErrorCategory = "Dracut_Operation"
	CategoryVerityOperation     ErrorCategory = "Verity_Operation"
)

// ErrorCode is a fine-grained, unique identifier for each specific failure
type ErrorCode string

const (
	// Default/unset error code
	CodeUnset ErrorCode = "Unset"

	// Invalid Input errors
	CodeInvalidOutputFormat            ErrorCode = "Invalid_Output_Format_Failure"
	CodeCannotCustomizePartitionsOnIso ErrorCode = "Partition_Customization_On_Iso_Failure"
	CodeCannotGenerateOutputFormat     ErrorCode = "Output_Format_Generation_Failure"
	CodeInvalidPackageListFile         ErrorCode = "Package_List_File_Validation_Failure"
	CodeInvalidPasswordFile            ErrorCode = "Password_File_Validation_Failure"
	CodeUnknownRpmSourceType           ErrorCode = "Rpm_Source_Type_Recognition_Failure"
	CodeRpmSourceTypeDetection         ErrorCode = "Rpm_Source_Type_Detection_Failure"

	// Image Conversion errors
	CodeImageFormatCheck ErrorCode = "Image_Format_Check_Failure"
	CodeQemuImgInfo      ErrorCode = "Qemu_Img_Info_Failure"

	// Filesystem Operation errors
	CodeHostnameWrite          ErrorCode = "Hostname_Write_Failure"
	CodeDirectoryCopy          ErrorCode = "Directory_Copy_Failure"
	CodeFileCopy               ErrorCode = "File_Copy_Failure"
	CodeOverlayDirectoryCreate ErrorCode = "Overlay_Directory_Create_Failure"
	CodeOverlayFstabUpdate     ErrorCode = "Overlay_Fstab_Update_Failure"
	CodeConfigDirMount         ErrorCode = "Config_Dir_Mount_Failure"
	CodeConfigDirUnmount       ErrorCode = "Config_Dir_Unmount_Failure"

	// Package Management errors
	CodePackageRepoMetadataRefresh ErrorCode = "Package_Repo_Metadata_Refresh_Failure"
	CodePackageInstall             ErrorCode = "Package_Install_Failure"
	CodePackageRemove              ErrorCode = "Package_Remove_Failure"
	CodePackageUpdate              ErrorCode = "Package_Update_Failure"
	CodePackageCacheClean          ErrorCode = "Package_Cache_Clean_Failure"
	CodePackageClean               ErrorCode = "Package_Clean_Failure"
	CodePackageUpgrade             ErrorCode = "Package_Upgrade_Failure"

	// Script Execution errors
	CodeScriptExecution ErrorCode = "Script_Execution_Failure"

	// Service Operation errors
	CodeServiceEnable  ErrorCode = "Service_Enable_Failure"
	CodeServiceDisable ErrorCode = "Service_Disable_Failure"

	// User/Group Operation errors
	CodeGroupGidSet    ErrorCode = "Group_Gid_Set_Failure"
	CodeUserUidSet     ErrorCode = "User_Uid_Set_Failure"
	CodeUserHomeDirSet ErrorCode = "User_Home_Dir_Set_Failure"
	CodeGroupExists    ErrorCode = "Group_Exists_Check_Failure"
	CodeGroupAdd       ErrorCode = "Group_Add_Failure"
	CodeUserExists     ErrorCode = "User_Exists_Check_Failure"
	CodeUserAdd        ErrorCode = "User_Add_Failure"
	CodeUserUpdate     ErrorCode = "User_Update_Failure"
	CodePasswordRead   ErrorCode = "Password_Read_Failure"

	// Miscellaneous errors for other operations
	CodeOverlayCheck     ErrorCode = "Overlay_Check_Failure"
	CodeRpmSourceMount   ErrorCode = "Rpm_Source_Mount_Failure"
	CodeInternalSystem   ErrorCode = "Internal_System_Failure"
	CodeNetworkOperation ErrorCode = "Network_Operation_Failure"
	CodePermissionDenied ErrorCode = "Permission_Denied_Failure"

	// Image History errors
	CodeImageHistoryDeepCopy        ErrorCode = "Image_History_Deep_Copy_Failure"
	CodeImageHistoryModify          ErrorCode = "Image_History_Modify_Failure"
	CodeImageHistoryDirectoryCreate ErrorCode = "Image_History_Directory_Create_Failure"
	CodeImageHistoryRead            ErrorCode = "Image_History_Read_Failure"
	CodeImageHistoryWrite           ErrorCode = "Image_History_Write_Failure"
	CodeImageHistoryFileCheck       ErrorCode = "Image_History_File_Check_Failure"
	CodeImageHistoryFileRead        ErrorCode = "Image_History_File_Read_Failure"
	CodeImageHistoryUnmarshal       ErrorCode = "Image_History_Unmarshal_Failure"
	CodeImageHistoryMarshal         ErrorCode = "Image_History_Marshal_Failure"
	CodeImageHistoryFileWrite       ErrorCode = "Image_History_File_Write_Failure"

	// Config Validation errors
	CodeConfigKdumpBootFiles    ErrorCode = "Config_Kdump_Boot_Files_Failure"
	CodeConfigKernelCommandLine ErrorCode = "Config_Kernel_Command_Line_Failure"
	CodeConfigBootstrapUrl      ErrorCode = "Config_Bootstrap_Url_Failure"
	CodeConfigIsoField          ErrorCode = "Config_Iso_Field_Failure"
	CodeConfigPxeField          ErrorCode = "Config_Pxe_Field_Failure"
	CodeConfigOsField           ErrorCode = "Config_Os_Field_Failure"
	CodeConfigDirectoryCreate   ErrorCode = "Config_Directory_Create_Failure"
	CodeConfigFilePersist       ErrorCode = "Config_File_Persist_Failure"
	CodeConfigFileExists        ErrorCode = "Config_File_Exists_Failure"
	CodeConfigFileLoad          ErrorCode = "Config_File_Load_Failure"

	// Filesystem Check errors
	CodeFilesystemE2fsckCheck    ErrorCode = "Filesystem_E2fsck_Check_Failure"
	CodeFilesystemXfsRepairCheck ErrorCode = "Filesystem_Xfs_Repair_Check_Failure"
	CodeFilesystemFsckCheck      ErrorCode = "Filesystem_Fsck_Check_Failure"

	// Release File errors
	CodeReleaseFileWrite ErrorCode = "Release_File_Write_Failure"

	// Filesystem Shrink errors
	CodeFilesystemSectorSizeGet ErrorCode = "Filesystem_Sector_Size_Get_Failure"
	CodeFilesystemShrink        ErrorCode = "Filesystem_Shrink_Failure"
	CodeFilesystemE2fsckResize  ErrorCode = "Filesystem_E2fsck_Resize_Failure"
	CodeFilesystemResize2fs     ErrorCode = "Filesystem_Resize2fs_Failure"

	// Partition Operation errors
	CodePartitionExtractAbsolutePath      ErrorCode = "Partition_Extract_Absolute_Path_Failure"
	CodePartitionExtractIntegrityCheck    ErrorCode = "Partition_Extract_Integrity_Check_Failure"
	CodePartitionExtractStatFile          ErrorCode = "Partition_Extract_Stat_File_Failure"
	CodePartitionExtractUnsupportedFormat ErrorCode = "Partition_Extract_Unsupported_Format_Failure"
	CodePartitionExtractMetadataConstruct ErrorCode = "Partition_Extract_Metadata_Construct_Failure"
	CodePartitionExtractRemoveRawFile     ErrorCode = "Partition_Extract_Remove_Raw_File_Failure"
	CodePartitionExtractRemoveTempFile    ErrorCode = "Partition_Extract_Remove_Temp_File_Failure"
	CodePartitionExtractCopyBlockDevice   ErrorCode = "Partition_Extract_Copy_Block_Device_Failure"
	CodePartitionExtractCompress          ErrorCode = "Partition_Extract_Compress_Failure"
	CodePartitionExtractOpenFile          ErrorCode = "Partition_Extract_Open_File_Failure"

	// UKI Operation errors
	CodeUKIPackageDependencyValidation ErrorCode = "UKI_Package_Dependency_Validation_Failure"
	CodeUKIDirectoryCreate             ErrorCode = "UKI_Directory_Create_Failure"
	CodeUKIShimFileCopy                ErrorCode = "UKI_Shim_File_Copy_Failure"
	CodeUKISystemdBootInstall          ErrorCode = "UKI_Systemd_Boot_Install_Failure"
	CodeUKIRandomSeedRemove            ErrorCode = "UKI_Random_Seed_Remove_Failure"
	CodeUKIKernelInitramfsMap          ErrorCode = "UKI_Kernel_Initramfs_Map_Failure"
	CodeUKIFileCopy                    ErrorCode = "UKI_File_Copy_Failure"
	CodeUKIKernelCmdlineExtract        ErrorCode = "UKI_Kernel_Cmdline_Extract_Failure"
	CodeUKICmdlineFileWrite            ErrorCode = "UKI_Cmdline_File_Write_Failure"

	// Dracut Operation errors
	CodeAddDracutDriver         ErrorCode = "Add_Dracut_Driver_Failure"
	CodeDracutConfigWrite       ErrorCode = "Dracut_Config_Write_Failure"
	CodeDracutConfigRead        ErrorCode = "Dracut_Config_Read_Failure"
	CodeDracutConfigAppend      ErrorCode = "Dracut_Config_Append_Failure"

	// Verity Operation errors
	CodeVerityPackageDependencyValidation ErrorCode = "Verity_Package_Dependency_Validation_Failure"
	CodeVerityDracutModuleAdd             ErrorCode = "Verity_Dracut_Module_Add_Failure"
	CodeVerityFstabUpdate                 ErrorCode = "Verity_Fstab_Update_Failure"
	CodeVerityGrubConfigPrepare           ErrorCode = "Verity_Grub_Config_Prepare_Failure"
	CodeVerityHashSignatureSupport        ErrorCode = "Verity_Hash_Signature_Support_Failure"
	CodeVerityFstabRead                   ErrorCode = "Verity_Fstab_Read_Failure"
	CodeVerityDracutScriptInstall         ErrorCode = "Verity_Dracut_Script_Install_Failure"
	CodeVerityDracutFileInstall           ErrorCode = "Verity_Dracut_File_Install_Failure"
	CodeVerityKernelArgumentGenerate      ErrorCode = "Verity_Kernel_Argument_Generate_Failure"

	// Partition Copy Operation errors
	CodePartitionCopyTargetOsDetermination ErrorCode = "Partition_Copy_Target_Os_Determination_Failure"
	CodePartitionCopyFilesToNewLayout      ErrorCode = "Partition_Copy_Files_To_New_Layout_Failure"
	CodePartitionCopyFiles                 ErrorCode = "Partition_Copy_Files_Failure"

	// Partition UUID Operation errors
	CodePartitionUuidResetFilesystem   ErrorCode = "Partition_Uuid_Reset_Filesystem_Failure"
	CodePartitionUuidUpdate            ErrorCode = "Partition_Uuid_Update_Failure"
	CodePartitionE2fsckCheck           ErrorCode = "Partition_E2fsck_Check_Failure"
	CodePartitionVfatIdGenerate        ErrorCode = "Partition_Vfat_Id_Generate_Failure"
	CodePartitionVerityNotImplemented  ErrorCode = "Partition_Verity_Not_Implemented_Failure"
	CodePartitionUnsupportedFilesystem ErrorCode = "Partition_Unsupported_Filesystem_Failure"

	// TDNF Snapshot Operation errors  
	CodeTdnfSnapshotTimeParse        ErrorCode = "Tdnf_Snapshot_Time_Parse_Failure"
	CodeTdnfConfigParse              ErrorCode = "Tdnf_Config_Parse_Failure"
	CodeTdnfConfigDirectoryCreate    ErrorCode = "Tdnf_Config_Directory_Create_Failure"
	CodeTdnfConfigWrite              ErrorCode = "Tdnf_Config_Write_Failure"
	CodeTdnfConfigCleanup            ErrorCode = "Tdnf_Config_Cleanup_Failure"

	// Bootloader Operation errors
	CodeBootloaderHardReset            ErrorCode = "Bootloader_Hard_Reset_Failure"
	CodeBootloaderKernelCommandLineAdd ErrorCode = "Bootloader_Kernel_Command_Line_Add_Failure"
	CodeBootloaderSelinuxModeGet       ErrorCode = "Bootloader_Selinux_Mode_Get_Failure"
	CodeBootloaderRootFilesystemFind   ErrorCode = "Bootloader_Root_Filesystem_Find_Failure"
	CodeBootloaderRootMountIdTypeGet   ErrorCode = "Bootloader_Root_Mount_Id_Type_Get_Failure"
	CodeBootloaderImageBootTypeGet     ErrorCode = "Bootloader_Image_Boot_Type_Get_Failure"
	CodeBootloaderDiskConfigure        ErrorCode = "Bootloader_Disk_Configure_Failure"
	CodeBootloaderRootMountFind        ErrorCode = "Bootloader_Root_Mount_Find_Failure"
	CodeBootloaderRootMountSourceParse ErrorCode = "Bootloader_Root_Mount_Source_Parse_Failure"
	CodeBootloaderVerityRootUnsupported ErrorCode = "Bootloader_Verity_Root_Unsupported_Failure"
	CodeBootloaderMountIdUnsupported   ErrorCode = "Bootloader_Mount_Id_Unsupported_Failure"

	// Artifact Handling errors
	CodeArtifactImageConnection      ErrorCode = "Artifact_Image_Connection_Failure"
	CodeArtifactPartitionMount       ErrorCode = "Artifact_Partition_Mount_Failure"
	CodeArtifactDirectoryRead        ErrorCode = "Artifact_Directory_Read_Failure"
	CodeArtifactFileCopy             ErrorCode = "Artifact_File_Copy_Failure"
	CodeArtifactFileWrite            ErrorCode = "Artifact_File_Write_Failure"
	CodeArtifactYamlWrite            ErrorCode = "Artifact_Yaml_Write_Failure"
	CodeArtifactYamlMarshal          ErrorCode = "Artifact_Yaml_Marshal_Failure"
	CodeArtifactConfigValidation     ErrorCode = "Artifact_Config_Validation_Failure"
	CodeArtifactPathResolution       ErrorCode = "Artifact_Path_Resolution_Failure"
	CodeArtifactPartitionUnmount     ErrorCode = "Artifact_Partition_Unmount_Failure"
	CodeArtifactImageConversion      ErrorCode = "Artifact_Image_Conversion_Failure"
	CodeArtifactImageConnectionClose ErrorCode = "Artifact_Image_Connection_Close_Failure"
	CodeArtifactUuidRead             ErrorCode = "Artifact_Uuid_Read_Failure"
	CodeArtifactUuidNotFound         ErrorCode = "Artifact_Uuid_Not_Found_Failure"
	CodeArtifactUuidParse            ErrorCode = "Artifact_Uuid_Parse_Failure"

	// Boot Customization errors
	CodeBootGrubMkconfigGeneration ErrorCode = "Boot_Grub_Mkconfig_Generation_Failure"

	// COSI Operation errors
	CodeCosiDirectoryCreate         ErrorCode = "Cosi_Directory_Create_Failure"
	CodeCosiBuildFile               ErrorCode = "Cosi_Build_File_Failure"
	CodeCosiMetadataPopulate        ErrorCode = "Cosi_Metadata_Populate_Failure"
	CodeCosiMetadataMarshal         ErrorCode = "Cosi_Metadata_Marshal_Failure"
	CodeCosiFileCreate              ErrorCode = "Cosi_File_Create_Failure"
	CodeCosiImageAdd                ErrorCode = "Cosi_Image_Add_Failure"
	CodeCosiVerityAdd               ErrorCode = "Cosi_Verity_Add_Failure"
	CodeCosiFileOpen                ErrorCode = "Cosi_File_Open_Failure"
	CodeCosiTarHeaderWrite          ErrorCode = "Cosi_Tar_Header_Write_Failure"
	CodeCosiImageWrite              ErrorCode = "Cosi_Image_Write_Failure"
	CodeCosiFileStat                ErrorCode = "Cosi_File_Stat_Failure"
	CodeCosiDirectoryError          ErrorCode = "Cosi_Directory_Error_Failure"
	CodeCosiSha384Calculate         ErrorCode = "Cosi_Sha384_Calculate_Failure"
	CodeCosiMetadataError           ErrorCode = "Cosi_Metadata_Error_Failure"
	CodeCosiVerityMetadataError     ErrorCode = "Cosi_Verity_Metadata_Error_Failure"
	CodeCosiVerityConfigMismatch    ErrorCode = "Cosi_Verity_Config_Mismatch_Failure"
	CodeCosiPackageListMissing      ErrorCode = "Cosi_Package_List_Missing_Failure"
	CodeCosiRpmOutputGet            ErrorCode = "Cosi_Rpm_Output_Get_Failure"
	CodeCosiRpmLineMalformed        ErrorCode = "Cosi_Rpm_Line_Malformed_Failure"
	CodeCosiBootloaderDetect        ErrorCode = "Cosi_Bootloader_Detect_Failure"
	CodeCosiSystemdBootExtract      ErrorCode = "Cosi_Systemd_Boot_Extract_Failure"
	CodeCosiUkiExtract              ErrorCode = "Cosi_Uki_Extract_Failure"
	CodeCosiBootEntryNotFound       ErrorCode = "Cosi_Boot_Entry_Not_Found_Failure"
	CodeCosiBootloaderUnsupported   ErrorCode = "Cosi_Bootloader_Unsupported_Failure"
	CodeCosiKernelCmdlineExtract    ErrorCode = "Cosi_Kernel_Cmdline_Extract_Failure"
	CodeCosiKernelNameInvalid       ErrorCode = "Cosi_Kernel_Name_Invalid_Failure"
	CodeCosiSystemdBootDirCheck     ErrorCode = "Cosi_Systemd_Boot_Dir_Check_Failure"
	CodeCosiSystemdBootEntryExtract ErrorCode = "Cosi_Systemd_Boot_Entry_Extract_Failure"
	CodeCosiEntryDirectoryRead      ErrorCode = "Cosi_Entry_Directory_Read_Failure"
	CodeCosiEntryFileRead           ErrorCode = "Cosi_Entry_File_Read_Failure"
	CodeCosiBootloaderUnknown       ErrorCode = "Cosi_Bootloader_Unknown_Failure"
	CodeCosiHashPartitionMissing    ErrorCode = "Cosi_Hash_Partition_Missing_Failure"
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
