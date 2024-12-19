// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
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
)

func addImageHistory(imageChroot *safechroot.Chroot, imageUuid string, baseConfigPath string, toolVersion string, buildTime string, config *imagecustomizerapi.Config) error {
	var err error
	logger.Log.Infof("Creating image customizer history file")

	// Deep copy the config to avoid modifying the original config
	configCopy, err := deepCopyConfig(config)
	if err != nil {
		return fmt.Errorf("failed to deep copy config while writing image history: %w", err)
	}

	err = modifyConfig(configCopy, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to modify config while writing image history: %w", err)
	}

	customizerLoggingDirPath := filepath.Join(imageChroot.RootDir(), customizerLoggingDir)
	err = os.MkdirAll(customizerLoggingDirPath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create customizer logging directory: %w", err)
	}
	imageHistoryFilePath := filepath.Join(customizerLoggingDirPath, historyFileName)

	var allImageHistory []ImageHistory
	err = readImageHistory(imageHistoryFilePath, &allImageHistory)
	if err != nil {
		return fmt.Errorf("failed to read image history: %w", err)
	}

	err = writeImageHistory(imageHistoryFilePath, allImageHistory, imageUuid, buildTime, toolVersion, configCopy)
	if err != nil {
		return fmt.Errorf("failed to write image history: %w", err)
	}

	return nil
}

func readImageHistory(imageHistoryFilePath string, allImageHistory *[]ImageHistory) error {
	exists, err := file.PathExists(imageHistoryFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}

	if exists {
		file, err := os.ReadFile(imageHistoryFilePath)
		if err != nil {
			return fmt.Errorf("error reading image history file: %w", err)
		}

		err = json.Unmarshal(file, &allImageHistory)
		if err != nil {
			return fmt.Errorf("error unmarshalling image history file: %w", err)
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
		return fmt.Errorf("failed to marshal image history: %w", err)
	}

	err = file.Write(string(jsonBytes), imageHistoryFilePath)
	if err != nil {
		return fmt.Errorf("failed to write image history to file: %w", err)
	}

	return nil
}

func deepCopyConfig(config *imagecustomizerapi.Config) (*imagecustomizerapi.Config, error) {
	configCopy := &imagecustomizerapi.Config{}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	err = json.Unmarshal(data, configCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return configCopy, nil
}

func modifyConfig(configCopy *imagecustomizerapi.Config, baseConfigPath string) error {
	var err error
	redactedString := "[redacted]"

	err = populateScriptsList(configCopy.Scripts, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate scripts list: %w", err)
	}

	err = populateAdditionalFiles(configCopy.OS.AdditionalFiles, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate additional files: %w", err)
	}

	err = populateAdditionalDirs(configCopy.OS.AdditionalDirs, baseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to populate additional dirs: %w", err)
	}

	err = redactSshPublicKeys(configCopy.OS.Users, redactedString)
	if err != nil {
		return fmt.Errorf("failed to redact ssh public keys: %w", err)
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
				return fmt.Errorf("error computing relative path for %s: %w", path, err)
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
			return fmt.Errorf("error walking directory %s: %w", dirPath, err)
		}
		configAdditionalDirs[i].SHA256HashMap = hashes
	}
	return nil
}

func populateAdditionalFiles(configAdditionalFiles imagecustomizerapi.AdditionalFileList, baseConfigPath string) error {
	for i := range configAdditionalFiles {
		if configAdditionalFiles[i].Source == "" {
			continue
		}
		absSourceFile := file.GetAbsPathWithBase(baseConfigPath, configAdditionalFiles[i].Source)
		hash, err := generateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		configAdditionalFiles[i].SHA256Hash = hash
	}
	return nil
}

func redactSshPublicKeys(configUsers []imagecustomizerapi.User, redactedString string) error {
	for i := range configUsers {
		user := configUsers[i]
		for j := range user.SSHPublicKeys {
			user.SSHPublicKeys[j] = redactedString
		}

	}
	return nil
}

func populateScriptsList(scripts imagecustomizerapi.Scripts, baseConfigPath string) error {
	for i := range scripts.PostCustomization {
		path := scripts.PostCustomization[i].Path
		if path == "" {
			continue
		}
		absSourceFile := file.GetAbsPathWithBase(baseConfigPath, path)
		hash, err := generateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		scripts.PostCustomization[i].SHA256Hash = hash
	}

	for i := range scripts.FinalizeCustomization {
		path := scripts.FinalizeCustomization[i].Path
		if path == "" {
			continue
		}
		absSourceFile := file.GetAbsPathWithBase(baseConfigPath, path)
		hash, err := generateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		scripts.FinalizeCustomization[i].SHA256Hash = hash

	}

	return nil
}

func generateSHA256(path string) (hash string, err error) {
	hash, err = file.GenerateSHA256(path)
	if err != nil {
		return "", fmt.Errorf("error generating SHA256 for %s: %w", path, err)
	}
	return hash, nil
}
