// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"
	"log"
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

func addImageHistory(imageChroot *safechroot.Chroot, imageUuid string, baseConfigPath string, toolVersion string, buildTime string, config *imagecustomizerapi.Config) error {
	var err error
	configCopy, err := deepCopyConfig(config)
	if err != nil {
		return err
	}
	err = modifyConfig(configCopy, baseConfigPath)
	if err != nil {
		return err
	}

	logger.Log.Infof("Creating image customizer history file")
	var allImageHistory []ImageHistory

	fmt.Println(imageChroot.RootDir())
	customizerLoggingDirPath := filepath.Join(".")
	// customizerLoggingDirPath := filepath.Join(imageChroot.RootDir(), "/usr/share/image-customizer")
	os.MkdirAll(customizerLoggingDirPath, 0755)

	imageHistoryFilePath := filepath.Join(customizerLoggingDirPath, "history.json")

	err = readImageHistory(imageHistoryFilePath, &allImageHistory)
	if err != nil {
		return err
	}

	err = writeImageHistory(imageHistoryFilePath, allImageHistory, imageUuid, buildTime, toolVersion, configCopy)
	if err != nil {
		return err
	}

	return nil
}

func readImageHistory(imageHistoryFilePath string, allImageHistory *[]ImageHistory) error {
	exists, err := file.PathExists(imageHistoryFilePath)
	if err != nil {
		return err
	}

	if exists {
		file, err := os.ReadFile(imageHistoryFilePath)
		if err != nil {
			log.Fatalf("Error reading file: %v", err)
		}

		// Unmarshal the file content into the data slice
		err = json.Unmarshal(file, &allImageHistory)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON: %v", err)
		}
	}
	return nil
}

func writeImageHistory(imageHistoryFilePath string, allImageHistory []ImageHistory, imageUuid string, buildTime string, toolVersion string, configCopy *imagecustomizerapi.Config) error {

	// Add the current image history to the list
	currentImageHistory := ImageHistory{
		BuildTime:   buildTime,
		ToolVersion: toolVersion,
		ImageUuid:   imageUuid,
		Config:      *configCopy,
	}
	allImageHistory = append(allImageHistory, currentImageHistory)

	jsonBytes, err := json.MarshalIndent(allImageHistory, "", " ")
	if err != nil {
		return err
	}
	file.Write(string(jsonBytes), imageHistoryFilePath)
	return nil
}

func deepCopyConfig(config *imagecustomizerapi.Config) (*imagecustomizerapi.Config, error) {
	configCopy := &imagecustomizerapi.Config{}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, configCopy)
	if err != nil {
		return nil, err
	}

	return configCopy, nil
}

func modifyConfig(configCopy *imagecustomizerapi.Config, baseConfigPath string) error {
	var err error
	err = populateScriptsList(configCopy.Scripts, baseConfigPath)
	if err != nil {
		return err
	}

	err = populateAdditionalFiles(configCopy.OS.AdditionalFiles, baseConfigPath)
	if err != nil {
		return err
	}

	err = populateAdditionalDirs(configCopy.OS.AdditionalDirs, baseConfigPath)
	if err != nil {
		return err
	}

	err = redactSshPublicKeys(configCopy.OS.Users)
	if err != nil {
		return err
	}
	return nil
}

func populateAdditionalDirs(configAdditionalDirs imagecustomizerapi.DirConfigList, baseConfigPath string) error {

	for i := range configAdditionalDirs {
		hashes := make(map[string]string)
		sourcePath := configAdditionalDirs[i].Source
		logger.Log.Infof("sourcePath: %s", sourcePath)
		dirPath := file.GetAbsPathWithBase(baseConfigPath, sourcePath)
		logger.Log.Infof("dirPath: %s", dirPath)

		destPath := configAdditionalDirs[i].Destination
		logger.Log.Infof("destPath: %s", destPath)
		// Walk the directory
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Compute the relative path with respect to dirPath
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return fmt.Errorf("error computing relative path for %s: %w", path, err)
			}
			logger.Log.Infof("relPath: %s", relPath)

			// Normalize the relative path to ensure consistency
			relPath = filepath.Clean(relPath)
			logger.Log.Infof("relPath after clean: %s", relPath)

			hash, err := file.GenerateSHA256(path)
			if err != nil {
				return fmt.Errorf("failed to generate SHA256 for file %s: %w", path, err)
			}

			hashes[relPath] = hash
			return nil
		})
		if err != nil {
			return err
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
		hash, err := file.GenerateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		configAdditionalFiles[i].SHA256Hash = hash
	}
	return nil
}

func redactSshPublicKeys(configUsers []imagecustomizerapi.User) error {
	redactedString := "[redacted]"
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
			// ignore entry if content is provided instead of path
			continue
		}
		absSourceFile := file.GetAbsPathWithBase(baseConfigPath, path)
		hash, err := file.GenerateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		scripts.PostCustomization[i].SHA256Hash = hash
	}
	for i := range scripts.FinalizeCustomization {
		path := scripts.FinalizeCustomization[i].Path
		if path == "" {
			// ignore entry if content is provided instead of path
			continue
		}
		absSourceFile := file.GetAbsPathWithBase(baseConfigPath, path)
		hash, err := file.GenerateSHA256(absSourceFile)
		if err != nil {
			return err
		}
		scripts.FinalizeCustomization[i].SHA256Hash = hash

	}

	return nil

}
