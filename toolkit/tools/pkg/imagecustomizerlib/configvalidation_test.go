// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"os"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfig_InvalidValidateOptions_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{"invalid-resource-type"},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Nil(t, rc)
	assert.ErrorContains(t, err, "invalid-resource-type")
}

func TestValidateConfig_InvalidCustomizeOptions_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir:          t.TempDir(),
		OutputImageFormat: "invalid-format",
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidOutputFormat)
}

func TestValidateConfig_InvalidConfig_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				// Setting both Path and Oci makes config invalid
				Path: "test.vhdx",
				Oci: &imagecustomizerapi.OciImage{
					Uri: "oci://example.com/image:tag",
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "must only specify one of")
}

func TestValidateConfig_MissingPreviewFeature_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
		// Not including PreviewFeaturePackageSnapshotTime
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir:            t.TempDir(),
		PackageSnapshotTime: "2024-01-01",
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrPackageSnapshotPreviewRequired)
}

func TestValidateConfig_InvalidRpmSource_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}

	// Create a real file with invalid extension
	tmpFile := baseConfigPath + "/invalid.xyz"
	err := os.WriteFile(tmpFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	customizeOptions := ImageCustomizerOptions{
		BuildDir:    t.TempDir(),
		RpmsSources: []string{tmpFile},
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrRpmSourceTypeUnknown)
}

func TestValidateConfig_InvalidScript_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Scripts: imagecustomizerapi.Scripts{
			PostCustomization: []imagecustomizerapi.Script{
				{Path: "/absolute/path/script.sh"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrScriptNotUnderConfigDir)
}

func TestValidateConfig_ScriptPathIsDirectory_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a directory instead of a file for the script
	scriptDir := baseConfigPath + "/script_dir"
	err := os.Mkdir(scriptDir, 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Scripts: imagecustomizerapi.Scripts{
			PostCustomization: []imagecustomizerapi.Script{
				{Path: "script_dir"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrScriptFileNotFile)
}

func TestValidateConfig_SelinuxPolicyPathIsFile_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a file (not a directory) to pass as selinux policy path
	tmpFile := baseConfigPath + "/selinux.txt"
	err := os.WriteFile(tmpFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir:                t.TempDir(),
		OutputSelinuxPolicyPath: tmpFile,
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrOutputSelinuxPolicyPathIsFileArg)
}

func TestValidateConfig_InvalidIsoAdditionalFiles_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Iso: &imagecustomizerapi.Iso{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "non-existent-file.txt",
					Destination: "/dest/file.txt",
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidAdditionalFilesSource)
}

func TestValidateConfig_InvalidPxeAdditionalFiles_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Pxe: &imagecustomizerapi.Pxe{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{
					Source:      "non-existent-pxe-file.txt",
					Destination: "/dest/file.txt",
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidAdditionalFilesSource)
}

func TestValidateConfig_InvalidBaseConfig_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureBaseConfigs,
		},
		BaseConfigs: []imagecustomizerapi.BaseConfig{
			{Path: "non-existent-base-config.yaml"},
		},
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "non-existent-base-config.yaml")
}

func TestValidateConfig_SSHKeyPathIsDirectory_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a directory instead of a file for the SSH key
	sshKeyDir := baseConfigPath + "/ssh_key_dir"
	err := os.Mkdir(sshKeyDir, 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name:              "testuser",
					SSHPublicKeyPaths: []string{"ssh_key_dir"},
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrSSHPublicKeyNotFile)
}

func TestValidateConfig_PasswordFileIsDirectory_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a directory instead of a file for the password file
	passwordDir := baseConfigPath + "/password_dir"
	err := os.Mkdir(passwordDir, 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "testuser",
					Password: &imagecustomizerapi.Password{
						Type:  imagecustomizerapi.PasswordTypePlainTextFile,
						Value: "password_dir",
					},
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrPasswordFileNotFile)
}

func TestValidateConfig_SelinuxPolicyPathNotDir_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir:                t.TempDir(),
		OutputSelinuxPolicyPath: "/nonexistent/path/to/selinux",
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrOutputSelinuxPolicyPathNotDirArg)
}

func TestValidateConfig_SelinuxPolicyConfigPathIsFile_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a file instead of a directory for selinux policy path in config
	selinuxFile := baseConfigPath + "/selinux_policy"
	err := os.WriteFile(selinuxFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureOutputSelinuxPolicy,
		},
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
			SelinuxPolicyPath: "selinux_policy",
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrOutputSelinuxPolicyPathIsFileConfig)
}

func TestValidateConfig_SelinuxPolicyConfigPathNotDir_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureOutputSelinuxPolicy,
		},
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
			SelinuxPolicyPath: "nonexistent_selinux_path",
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrOutputSelinuxPolicyPathNotDirConfig)
}

func TestValidateConfig_SelinuxPolicyConfigPathValid_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a valid directory for selinux policy path
	selinuxDir := baseConfigPath + "/selinux_policy"
	err := os.Mkdir(selinuxDir, 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureOutputSelinuxPolicy,
		},
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
			SelinuxPolicyPath: "selinux_policy",
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_InvalidPackageRemoveList_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				RemoveLists: []string{"nonexistent-remove-list.txt"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "nonexistent-remove-list.txt")
}

func TestValidateConfig_InvalidPackageInstallList_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				InstallLists: []string{"nonexistent-install-list.txt"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "nonexistent-install-list.txt")
}

func TestValidateConfig_InvalidPackageUpdateList_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				UpdateLists: []string{"nonexistent-update-list.txt"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "nonexistent-update-list.txt")
}

func TestValidateConfig_AdditionalDirsSourceIsFile_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a file instead of a directory
	sourceFile := baseConfigPath + "/source_file"
	err := os.WriteFile(sourceFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			AdditionalDirs: imagecustomizerapi.DirConfigList{
				{
					Source:      "source_file",
					Destination: "/dest",
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrAdditionalDirsSourceIsFile)
}

func TestValidateConfig_AdditionalDirsSourceNotFound_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			AdditionalDirs: imagecustomizerapi.DirConfigList{
				{
					Source:      "nonexistent_dir",
					Destination: "/dest",
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrAdditionalDirsSourceNotFound)
}

func TestValidateConfig_OutputImageFileIsDirectory_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a directory to be used as output image file (invalid)
	outputDir := baseConfigPath + "/output_dir"
	err := os.Mkdir(outputDir, 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output_dir",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrOutputImageFileIsDirectory)
}

func TestValidateConfigWithConfigFileOptions_MinimalConfig_Pass(t *testing.T) {
	ctx := context.Background()
	configDir := t.TempDir()
	configFile := configDir + "/config.yaml"
	configContent := `
input:
  image:
    path: test.vhdx
output:
  image:
    path: output.vhdx
    format: vhdx
`
	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	assert.NoError(t, err)

	options := ValidateConfigOptions{}

	err = ValidateConfigWithConfigFileOptions(ctx, configFile, options)

	assert.NoError(t, err)
}

func TestValidateConfigWithConfigFileOptions_InvalidYaml_Fail(t *testing.T) {
	ctx := context.Background()
	configDir := t.TempDir()
	configFile := configDir + "/config.yaml"
	configContent := `invalid yaml: [[[`
	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	assert.NoError(t, err)

	options := ValidateConfigOptions{}

	err = ValidateConfigWithConfigFileOptions(ctx, configFile, options)

	assert.Error(t, err)
}

func TestValidateConfigWithConfigFileOptions_InvalidConfig_Fail(t *testing.T) {
	ctx := context.Background()
	configDir := t.TempDir()
	configFile := configDir + "/config.yaml"
	configContent := `
input:
  image:
    path: test.vhdx
    oci:
      uri: oci://example.com/image:tag
output:
  image:
    path: output.vhdx
    format: vhdx
`
	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	assert.NoError(t, err)

	options := ValidateConfigOptions{}

	err = ValidateConfigWithConfigFileOptions(ctx, configFile, options)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidImageConfig)
}

func TestValidateConfigWithConfigFileOptions_EmptyConfig_Pass(t *testing.T) {
	ctx := context.Background()
	configDir := t.TempDir()
	configFile := configDir + "/config.yaml"
	// Minimal config that relies on defaults for input.image.path, output.image.format, and output.image.path
	configContent := `---
`
	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	assert.NoError(t, err)

	options := ValidateConfigOptions{}

	err = ValidateConfigWithConfigFileOptions(ctx, configFile, options)

	assert.NoError(t, err)
}

func TestValidateConfigWithConfigFileOptions_DeletedWorkingDir_Fail(t *testing.T) {
	ctx := context.Background()

	// Save original working directory
	originalDir, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create parent directory with config file
	parentDir, err := os.MkdirTemp("", "test-parent-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(parentDir)

	configFile := parentDir + "/config.yaml"
	configContent := `---
`
	err = os.WriteFile(configFile, []byte(configContent), 0o644)
	assert.NoError(t, err)

	// Create subdirectory to change into and delete
	toDeleteDir := parentDir + "/to-delete"
	err = os.Mkdir(toDeleteDir, 0o755)
	assert.NoError(t, err)

	// Change into the subdirectory
	err = os.Chdir(toDeleteDir)
	assert.NoError(t, err)

	// Delete the subdirectory while we're in it
	err = os.RemoveAll(toDeleteDir)
	assert.NoError(t, err)

	options := ValidateConfigOptions{}

	// Use "../config.yaml". The file is accessible via .., but filepath.Abs("../") fails
	// because Getwd() fails when the current directory is deleted
	err = ValidateConfigWithConfigFileOptions(ctx, "../config.yaml", options)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrGetAbsoluteConfigPath)
}

func TestValidateConfig_DeletedWorkingDir_Fail(t *testing.T) {
	ctx := context.Background()

	// Save original working directory
	originalDir, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "test-deleted-dir")
	assert.NoError(t, err)

	// Change into the temp directory and delete it
	err = os.Chdir(tempDir)
	assert.NoError(t, err)
	err = os.RemoveAll(tempDir)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{}

	// filepath.Abs will fail because the working directory no longer exists
	rc, err := ValidateConfig(ctx, ".", config, true, validateOptions, customizeOptions)

	assert.Nil(t, rc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getwd")
}

func TestResolveCosiCompressionLevel_EmptyCosi_Pass(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, imagecustomizerapi.DefaultCosiCompressionLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_EmptyBareMetalImage_Pass(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeBareMetalImage)

	assert.Equal(t, imagecustomizerapi.DefaultBareMetalCosiCompressionLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_EmptyOtherFormat_Pass(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeVhdx)

	assert.Equal(t, imagecustomizerapi.DefaultCosiCompressionLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_SingleConfigCosi_Pass(t *testing.T) {
	configLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &configLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, configLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_SingleConfigBareMetalImage_Pass(t *testing.T) {
	configLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &configLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(
		configChain, nil, imagecustomizerapi.ImageFormatTypeBareMetalImage)

	assert.Equal(t, configLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_CurrentConfigOverridesBase_Pass(t *testing.T) {
	baseConfigLevel := 9
	currConfigLevel := 22
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &baseConfigLevel,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &currConfigLevel,
							},
						},
					},
				},
			},
		},
	}

	compression := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, currConfigLevel, compression)
}

func TestResolveCosiCompressionLevel_CLIOverridesConfig_Pass(t *testing.T) {
	configLevel := 22
	cliLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &configLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, &cliLevel, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, cliLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_CLIOverridesBaseConfig_Pass(t *testing.T) {
	baseConfigLevel := 9
	currConfigLevel := 22
	cliLevel := 15
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &baseConfigLevel,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &currConfigLevel,
							},
						},
					},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, &cliLevel, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, cliLevel, resolvedLevel)
}

func TestResolveCosiCompressionLevel_OnlyBaseConfigCompressionLevel_Pass(t *testing.T) {
	// Test the scenario described in the design doc:
	// "Inheriting compression without the preview feature in current config"
	level := 19
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{
						Cosi: imagecustomizerapi.CosiConfig{
							Compression: imagecustomizerapi.CosiCompression{
								Level: &level,
							},
						},
					},
				},
			},
		},
		{
			Config: &imagecustomizerapi.Config{
				Output: imagecustomizerapi.Output{
					Image: imagecustomizerapi.OutputImage{},
				},
			},
		},
	}

	resolvedLevel := resolveCosiCompressionLevel(configChain, nil, imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, 19, resolvedLevel)
}

func TestDefaultCosiCompressionLong_Cosi_Pass(t *testing.T) {
	resolvedLong := defaultCosiCompressionLong(imagecustomizerapi.ImageFormatTypeCosi)

	assert.Equal(t, imagecustomizerapi.DefaultCosiCompressionLong, resolvedLong)
}

func TestDefaultCosiCompressionLong_BareMetalImage_Pass(t *testing.T) {
	resolvedLong := defaultCosiCompressionLong(imagecustomizerapi.ImageFormatTypeBareMetalImage)

	assert.Equal(t, imagecustomizerapi.DefaultBareMetalCosiCompressionLong, resolvedLong)
}

func TestResolveIsoAdditionalFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_SingleConfig(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	content := "test content"
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "files/a.txt",
							Destination: "/a.txt",
							Permissions: &perms,
						},
						{
							Content:     &content,
							Destination: "/b.txt",
						},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/config/files/a.txt",
			Destination: "/a.txt",
			Permissions: &perms,
		},
		{
			Content:     &content,
			Destination: "/b.txt",
		},
	}, result.AdditionalFiles)
}

func TestResolveIsoAdditionalFiles_MultipleConfigs(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "base-files/base.txt",
							Destination: "/base.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "current-files/current.txt",
							Destination: "/current.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/base-files/base.txt",
			Destination: "/base.txt",
			Permissions: &perms,
		},
		{
			Source:      "/current/current-files/current.txt",
			Destination: "/current.txt",
			Permissions: &perms,
		},
	}, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_SingleConfig(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	content := "test content"
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "files/a.txt",
							Destination: "/a.txt",
							Permissions: &perms,
						},
						{
							Content:     &content,
							Destination: "/b.txt",
						},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/config/files/a.txt",
			Destination: "/a.txt",
			Permissions: &perms,
		},
		{
			Content:     &content,
			Destination: "/b.txt",
		},
	}, result.AdditionalFiles)
}

func TestResolvePxeAdditionalFiles_MultipleConfigs(t *testing.T) {
	perms := imagecustomizerapi.FilePermissions(0o644)
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "base-files/base.txt",
							Destination: "/base.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					AdditionalFiles: imagecustomizerapi.AdditionalFileList{
						{
							Source:      "current-files/current.txt",
							Destination: "/current.txt",
							Permissions: &perms,
						},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Base config's files should come first, then current config's files
	// Paths should be resolved relative to each config's base path
	assert.Equal(t, imagecustomizerapi.AdditionalFileList{
		{
			Source:      "/base/base-files/base.txt",
			Destination: "/base.txt",
			Permissions: &perms,
		},
		{
			Source:      "/current/current-files/current.txt",
			Destination: "/current.txt",
			Permissions: &perms,
		},
	}, result.AdditionalFiles)
}

func TestResolveIsoKernelCommandLine_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolveIsoConfig(configChain)

	assert.Equal(t, []string{"console=tty0", "console=ttyS0"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_MultipleConfigs(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.info", "rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	// Base config's args should come first, then current config's args are appended
	assert.Equal(t, []string{"console=tty0", "rd.info", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoKernelCommandLine_EmptyArgsInMiddle(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{}, // Empty args in the middle config
					},
				},
			},
			BaseConfigPath: "/middle",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)

	// Should skip the empty config in the middle
	assert.Equal(t, []string{"console=tty0", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: nil,
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Empty(t, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0", "console=ttyS0"},
					},
				},
			},
			BaseConfigPath: "/base/config",
		},
	}

	result := resolvePxeConfig(configChain)

	assert.Equal(t, []string{"console=tty0", "console=ttyS0"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_MultipleConfigs(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.info", "rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Base config's args should come first, then current config's args are appended
	assert.Equal(t, []string{"console=tty0", "rd.info", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolvePxeKernelCommandLine_EmptyArgsInMiddle(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"console=tty0"},
					},
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{}, // Empty args in the middle config
					},
				},
			},
			BaseConfigPath: "/middle",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KernelCommandLine: imagecustomizerapi.KernelCommandLine{
						ExtraCommandLine: []string{"rd.shell"},
					},
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)

	// Should skip the empty config in the middle
	assert.Equal(t, []string{"console=tty0", "rd.shell"}, result.KernelCommandLine.ExtraCommandLine)
}

func TestResolveIsoInitramfsType_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolveIsoInitramfsType_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolveIsoInitramfsType_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolveIsoInitramfsType_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolveIsoInitramfsType_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					InitramfsType: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolvePxeInitramfsType_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolvePxeInitramfsType_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageType(""), result.InitramfsType)
}

func TestResolvePxeInitramfsType_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolvePxeInitramfsType_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeBootstrap,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeBootstrap, result.InitramfsType)
}

func TestResolvePxeInitramfsType_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: imagecustomizerapi.InitramfsImageTypeFullOS,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					InitramfsType: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, imagecustomizerapi.InitramfsImageTypeFullOS, result.InitramfsType)
}

func TestResolveIsoKdumpBootFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolveIsoConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_NilIso(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolveIsoConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_SingleConfig(t *testing.T) {
	kdumpType := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpType,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpType, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_OverrideFromCurrent(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	kdumpNone := imagecustomizerapi.KdumpBootFilesTypeNone
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpNone,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpNone, result.KdumpBootFiles)
}

func TestResolveIsoKdumpBootFiles_NilInCurrentUsesBase(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Iso: &imagecustomizerapi.Iso{
					KdumpBootFiles: nil, // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolveIsoConfig(configChain)
	assert.Equal(t, &kdumpKeep, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Nil(t, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_SingleConfig(t *testing.T) {
	kdumpType := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpType,
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpType, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_OverrideFromCurrent(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	kdumpNone := imagecustomizerapi.KdumpBootFilesTypeNone
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpNone,
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpNone, result.KdumpBootFiles)
}

func TestResolvePxeKdumpBootFiles_NilInCurrentUsesBase(t *testing.T) {
	kdumpKeep := imagecustomizerapi.KdumpBootFilesTypeKeep
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: &kdumpKeep,
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					KdumpBootFiles: nil, // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, &kdumpKeep, result.KdumpBootFiles)
}

func TestResolvePxeBootstrapBaseUrl_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://base.example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://current.example.com/pxe/",
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://current.example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapBaseUrl_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "http://base.example.com/pxe/",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapBaseUrl: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://base.example.com/pxe/", result.BootstrapBaseUrl)
}

func TestResolvePxeBootstrapFileUrl_Empty(t *testing.T) {
	configChain := []*ConfigWithBasePath{}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_NilPxe(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config:         &imagecustomizerapi.Config{},
			BaseConfigPath: "/base",
		},
	}
	result := resolvePxeConfig(configChain)
	assert.Equal(t, "", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_SingleConfig(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://example.com/pxe/image.iso",
				},
			},
			BaseConfigPath: "/base",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://example.com/pxe/image.iso", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_OverrideFromCurrent(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://base.example.com/pxe/base.iso",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://current.example.com/pxe/current.iso",
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://current.example.com/pxe/current.iso", result.BootstrapFileUrl)
}

func TestResolvePxeBootstrapFileUrl_UnspecifiedInCurrentUsesBase(t *testing.T) {
	configChain := []*ConfigWithBasePath{
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "http://base.example.com/pxe/base.iso",
				},
			},
			BaseConfigPath: "/base",
		},
		{
			Config: &imagecustomizerapi.Config{
				Pxe: &imagecustomizerapi.Pxe{
					BootstrapFileUrl: "", // Unspecified
				},
			},
			BaseConfigPath: "/current",
		},
	}

	result := resolvePxeConfig(configChain)
	assert.Equal(t, "http://base.example.com/pxe/base.iso", result.BootstrapFileUrl)
}

func TestValidateConfigPostImageDownload_IsoInputVhdxOutput_Fail(t *testing.T) {
	rc := &ResolvedConfig{
		InputImage: imagecustomizerapi.InputImage{
			Path: "input.iso",
		},
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhdx,
		Config:            &imagecustomizerapi.Config{},
	}

	err := ValidateConfigPostImageDownload(rc)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotGenerateOutputFormat)
}

func TestValidateConfigPostImageDownload_NonIsoInput_Pass(t *testing.T) {
	rc := &ResolvedConfig{
		InputImage: imagecustomizerapi.InputImage{
			Path: "input.vhdx",
		},
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeVhdx,
		Config:            &imagecustomizerapi.Config{},
	}

	err := ValidateConfigPostImageDownload(rc)

	assert.NoError(t, err)
}

func TestValidateConfigPostImageDownload_IsoInputIsoOutput_Pass(t *testing.T) {
	rc := &ResolvedConfig{
		InputImage: imagecustomizerapi.InputImage{
			Path: "input.iso",
		},
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeIso,
		Config:            &imagecustomizerapi.Config{},
	}

	err := ValidateConfigPostImageDownload(rc)

	assert.NoError(t, err)
}

func TestValidateConfigPostImageDownload_IsoInputWithPartitions_Fail(t *testing.T) {
	rc := &ResolvedConfig{
		InputImage: imagecustomizerapi.InputImage{
			Path: "input.iso",
		},
		OutputImageFormat: imagecustomizerapi.ImageFormatTypeIso,
		Config: &imagecustomizerapi.Config{
			Storage: imagecustomizerapi.Storage{
				Disks: []imagecustomizerapi.Disk{
					{
						Partitions: []imagecustomizerapi.Partition{
							{Id: "root"},
						},
					},
				},
			},
		},
	}

	err := ValidateConfigPostImageDownload(rc)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotCustomizePartitionsOnIso)
}

func TestValidateConfig_PasswordFileNotFound_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name: "testuser",
					Password: &imagecustomizerapi.Password{
						Type:  imagecustomizerapi.PasswordTypePlainTextFile,
						Value: "nonexistent_password_file.txt",
					},
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidPasswordFile)
}

func TestValidateConfig_SSHKeyPathNotFound_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name:              "testuser",
					SSHPublicKeyPaths: []string{"nonexistent_key.pub"},
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidSSHPublicKeyFile)
}

func TestValidateConfig_ValidUserFiles_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create valid SSH key file
	sshKeyFile := baseConfigPath + "/id_rsa.pub"
	err := os.WriteFile(sshKeyFile, []byte("ssh-rsa AAAAB3..."), 0o644)
	assert.NoError(t, err)

	// Create valid password file
	passwordFile := baseConfigPath + "/password.txt"
	err = os.WriteFile(passwordFile, []byte("testpassword"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Users: []imagecustomizerapi.User{
				{
					Name:              "testuser",
					SSHPublicKeyPaths: []string{"id_rsa.pub"},
					Password: &imagecustomizerapi.Password{
						Type:  imagecustomizerapi.PasswordTypePlainTextFile,
						Value: "password.txt",
					},
				},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_InvalidFinalizeScript_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Scripts: imagecustomizerapi.Scripts{
			FinalizeCustomization: []imagecustomizerapi.Script{
				{Path: "/absolute/path/script.sh"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrInvalidFinalizeScript)
}

func TestValidateConfig_ScriptFileNotFound_Fail(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()
	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Scripts: imagecustomizerapi.Scripts{
			PostCustomization: []imagecustomizerapi.Script{
				{Path: "nonexistent_script.sh"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, ErrScriptFileNotReadable)
}

func TestValidateConfig_ValidScripts_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create valid script files
	err := os.WriteFile(baseConfigPath+"/post.sh", []byte("#!/bin/bash\necho post"), 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(baseConfigPath+"/finalize.sh", []byte("#!/bin/bash\necho finalize"), 0o755)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Scripts: imagecustomizerapi.Scripts{
			PostCustomization: []imagecustomizerapi.Script{
				{Path: "post.sh"},
			},
			FinalizeCustomization: []imagecustomizerapi.Script{
				{Path: "finalize.sh"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_ValidIsoAdditionalFiles_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a valid additional file
	additionalFile := baseConfigPath + "/additional.txt"
	err := os.WriteFile(additionalFile, []byte("test content"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Iso: &imagecustomizerapi.Iso{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{Source: "additional.txt", Destination: "/etc/additional.txt"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.iso",
				Format: imagecustomizerapi.ImageFormatTypeIso,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_ValidPxeAdditionalFiles_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a valid additional file
	additionalFile := baseConfigPath + "/pxe_additional.txt"
	err := os.WriteFile(additionalFile, []byte("test content"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		Pxe: &imagecustomizerapi.Pxe{
			AdditionalFiles: imagecustomizerapi.AdditionalFileList{
				{Source: "pxe_additional.txt", Destination: "/etc/pxe_additional.txt"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output",
				Format: imagecustomizerapi.ImageFormatTypePxeDir,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_ValidAdditionalDirs_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a valid source directory with a file
	sourceDir := baseConfigPath + "/source_dir"
	err := os.Mkdir(sourceDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(sourceDir+"/test.txt", []byte("test"), 0o644)
	assert.NoError(t, err)

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			AdditionalDirs: imagecustomizerapi.DirConfigList{
				{Source: "source_dir", Destination: "/opt/source"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{
		ValidateResources: imagecustomizerapi.ValidateResourceTypes{imagecustomizerapi.ValidateResourceTypeFiles},
	}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_PackageInstallWithRpmSource_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	// Create a simple rpm sources directory
	rpmDir := t.TempDir()

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			Packages: imagecustomizerapi.Packages{
				Install: []string{"vim"},
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir:    t.TempDir(),
		RpmsSources: []string{rpmDir},
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
}

func TestValidateConfig_BootLoaderHardReset_Pass(t *testing.T) {
	ctx := context.Background()
	baseConfigPath := t.TempDir()

	config := &imagecustomizerapi.Config{
		Input: imagecustomizerapi.Input{
			Image: imagecustomizerapi.InputImage{
				Path: "test.vhdx",
			},
		},
		OS: &imagecustomizerapi.OS{
			BootLoader: imagecustomizerapi.BootLoader{
				ResetType: imagecustomizerapi.ResetBootLoaderTypeHard,
			},
		},
		Output: imagecustomizerapi.Output{
			Image: imagecustomizerapi.OutputImage{
				Path:   "output.vhdx",
				Format: imagecustomizerapi.ImageFormatTypeVhdx,
			},
		},
	}
	validateOptions := ValidateConfigOptions{}
	customizeOptions := ImageCustomizerOptions{
		BuildDir: t.TempDir(),
	}

	rc, err := ValidateConfig(ctx, baseConfigPath, config, true, validateOptions, customizeOptions)

	assert.NoError(t, err)
	assert.NotNil(t, rc)
	assert.Equal(t, imagecustomizerapi.ResetBootLoaderTypeHard, rc.BootLoader.ResetType)
}
