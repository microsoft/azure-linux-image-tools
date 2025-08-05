// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"go.opentelemetry.io/otel"
)

var (
	// Image history errors
	ErrImageHistoryDeepCopy        = NewImageCustomizerError("ImageHistory:DeepCopy", "failed to deep copy config")
	ErrImageHistoryModify          = NewImageCustomizerError("ImageHistory:Modify", "failed to modify config")
	ErrImageHistoryDirectoryCreate = NewImageCustomizerError("ImageHistory:DirectoryCreate", "failed to create logging directory")
	ErrImageHistoryRead            = NewImageCustomizerError("ImageHistory:Read", "failed to read image history")
	ErrImageHistoryWrite           = NewImageCustomizerError("ImageHistory:Write", "failed to write image history")
	ErrImageHistoryFileCheck       = NewImageCustomizerError("ImageHistory:FileCheck", "failed to check if file exists")
	ErrImageHistoryFileRead        = NewImageCustomizerError("ImageHistory:FileRead", "failed to read image history file")
	ErrImageHistoryUnmarshal       = NewImageCustomizerError("ImageHistory:Unmarshal", "failed to unmarshal image history file")
	ErrImageHistoryMarshal         = NewImageCustomizerError("ImageHistory:Marshal", "failed to marshal image history")
	ErrImageHistoryFileWrite       = NewImageCustomizerError("ImageHistory:FileWrite", "failed to write image history to file")
)

type ImageHistory struct {
	BuildTime   string                    `yaml:"timestamp" json:"timestamp"`
	ToolVersion string                    `yaml:"toolVersion" json:"toolVersion"`
	ImageUuid   string                    `yaml:"imageUuid" json:"imageUuid"`
	Config      imagecustomizerapi.Config `yaml:"config" json:"config"`
}

const (
	customizerLoggingDir = "/usr/share/image-customizer"
	historyFileName      = "history.json"
	redactedString       = "[redacted]"
)

func addImageHistory(ctx context.Context, imageChroot *safechroot.Chroot, imageUuid string,
	baseConfigPath string, toolVersion string, buildTime string, config *imagecustomizerapi.Config,
) error {
	cannotWriteHistoryFile := isPathOnReadOnlyMount(customizerLoggingDir, imageChroot)
	if cannotWriteHistoryFile {
		return nil
	}

	err := addImageHistoryHelper(ctx, imageChroot.RootDir(), imageUuid, baseConfigPath, toolVersion,
		buildTime, config)
	if err != nil {
		return err
	}

	return nil
}

func addImageHistoryHelper(ctx context.Context, rootDir string, imageUuid string, baseConfigPath string,
	toolVersion string, buildTime string, config *imagecustomizerapi.Config,
) error {
	var err error
	logger.Log.Infof("Creating image customizer history file")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "add_image_history")
	defer span.End()

	// Deep copy the config to avoid modifying the original config
	configCopy, err := deepCopyConfig(config)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrImageHistoryDeepCopy, err)
	}

	err = modifyConfig(configCopy, baseConfigPath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrImageHistoryModify, err)
	}

	customizerLoggingDirPath := filepath.Join(rootDir, customizerLoggingDir)
	err = os.MkdirAll(customizerLoggingDirPath, 0o755)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrImageHistoryDirectoryCreate, customizerLoggingDirPath, err)
	}
	imageHistoryFilePath := filepath.Join(customizerLoggingDirPath, historyFileName)

	var allImageHistory []ImageHistory
	err = readImageHistory(imageHistoryFilePath, &allImageHistory)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrImageHistoryRead, imageHistoryFilePath, err)
	}

	err = writeImageHistory(imageHistoryFilePath, allImageHistory, imageUuid, buildTime, toolVersion, configCopy)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrImageHistoryWrite, imageHistoryFilePath, err)
	}

	return nil
}

func readImageHistory(imageHistoryFilePath string, allImageHistory *[]ImageHistory) error {
	exists, err := file.PathExists(imageHistoryFilePath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrImageHistoryFileCheck, err)
	}

	if exists {
		file, err := os.ReadFile(imageHistoryFilePath)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrImageHistoryFileRead, err)
		}

		err = json.Unmarshal(file, &allImageHistory)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrImageHistoryUnmarshal, err)
		}
	}
	return nil
}

func writeImageHistory(imageHistoryFilePath string, allImageHistory []ImageHistory, imageUuid string, buildTime string, toolVersion string, configCopy *imagecustomizerapi.Config) error {
	currentImageHistory := ImageHistory{
		BuildTime:   buildTime,
		ToolVersion: toolVersion,
		ImageUuid:   imageUuid,
		Config:      *configCopy,
	}
	allImageHistory = append(allImageHistory, currentImageHistory)

	jsonBytes, err := json.MarshalIndent(allImageHistory, "", " ")
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrImageHistoryMarshal, err)
	}

	err = file.Write(string(jsonBytes), imageHistoryFilePath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrImageHistoryFileWrite, err)
	}

	return nil
}

func deepCopyConfig(config *imagecustomizerapi.Config) (*imagecustomizerapi.Config, error) {
	configCopy := &imagecustomizerapi.Config{}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config:\n%w", err)
	}

	err = json.Unmarshal(data, configCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config:\n%w", err)
	}

	return configCopy, nil
}

func modifyConfig(configCopy *imagecustomizerapi.Config, baseConfigPath string) error {
	var err error

	err = populateScriptsList(configCopy.Scripts, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate scripts list:\n%w", err)
	}

	err = populateAdditionalFiles(configCopy.OS.AdditionalFiles, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate additional files:\n%w", err)
	}

	err = populateAdditionalDirs(configCopy.OS.AdditionalDirs, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate additional dirs:\n%w", err)
	}

	err = redactSshPublicKeys(configCopy.OS.Users)
	if err != nil {
		return fmt.Errorf("failed to redact ssh public keys:\n%w", err)
	}
	return nil
}

func populateAdditionalDirs(configAdditionalDirs imagecustomizerapi.DirConfigList, baseConfigPath string) error {
	for i := range configAdditionalDirs {
		hashes := make(map[string]string)
		sourcePath := configAdditionalDirs[i].Source
		dirPath := file.GetAbsPathWithBase(baseConfigPath, sourcePath)

		addFileHashToMap := func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return fmt.Errorf("error computing relative path for %s:\n%w", path, err)
			}

			hash, err := generateSHA256(path)
			if err != nil {
				return err
			}

			hashes[relPath] = hash
			return nil
		}

		err := filepath.WalkDir(dirPath, addFileHashToMap)
		if err != nil {
			return fmt.Errorf("error walking directory %s:\n%w", dirPath, err)
		}
		configAdditionalDirs[i].SHA256HashMap = hashes
	}
	return nil
}

func populateAdditionalFiles(configAdditionalFiles imagecustomizerapi.AdditionalFileList, baseConfigPath string) error {
	for i := range configAdditionalFiles {
		if configAdditionalFiles[i].Source != "" {
			absSourceFile := file.GetAbsPathWithBase(baseConfigPath, configAdditionalFiles[i].Source)
			hash, err := generateSHA256(absSourceFile)
			if err != nil {
				return err
			}
			configAdditionalFiles[i].SHA256Hash = hash
		}
	}
	return nil
}

func redactSshPublicKeys(configUsers []imagecustomizerapi.User) error {
	for i := range configUsers {
		user := configUsers[i]
		for j := range user.SSHPublicKeys {
			user.SSHPublicKeys[j] = redactedString
		}

	}
	return nil
}

func populateScriptsList(scripts imagecustomizerapi.Scripts, baseConfigPath string) error {
	if err := processScripts(scripts.PostCustomization, baseConfigPath); err != nil {
		return fmt.Errorf("error processing PostCustomization scripts:\n%w", err)
	}
	if err := processScripts(scripts.FinalizeCustomization, baseConfigPath); err != nil {
		return fmt.Errorf("error processing FinalizeCustomization scripts:\n%w", err)
	}
	return nil
}

func processScripts(scripts []imagecustomizerapi.Script, baseConfigPath string) error {
	for i := range scripts {
		path := scripts[i].Path
		if path != "" {
			absSourceFile := file.GetAbsPathWithBase(baseConfigPath, path)
			hash, err := generateSHA256(absSourceFile)
			if err != nil {
				return err
			}
			scripts[i].SHA256Hash = hash
		}
	}
	return nil
}

func generateSHA256(path string) (hash string, err error) {
	hash, err = file.GenerateSHA256(path)
	if err != nil {
		return "", fmt.Errorf("error generating SHA256 for %s:\n%w", path, err)
	}
	return hash, nil
}
