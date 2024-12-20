package imagecustomizerlib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func createTestConfig(configFilePath string, t *testing.T) imagecustomizerapi.Config {
	configFile := filepath.Join(testDir, configFilePath)

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)
	return config
}

func TestAddImageHistory(t *testing.T) {
	tempDir := filepath.Join(tmpDir, "TestAddImageHistory")
	chroot := safechroot.NewChroot(tempDir, false)
	err := chroot.Initialize("", []string{}, []*safechroot.MountPoint{}, false)
	assert.NoError(t, err)
	defer chroot.Close(false)

	historyDir := filepath.Join(tempDir, customizerLoggingDir)
	historyFilePath := filepath.Join(historyDir, historyFileName)
	config := createTestConfig("imagehistory-config.yaml", t)
	// Serialize the config before calling addImageHistory
	originalConfigBytes, err := yaml.Marshal(config)
	assert.NoError(t, err, "failed to serialize original config")

	expectedVersion := "0.1.0"
	expectedDate := time.Now().Format("2006-01-02T15:04:05Z")
	_, expectedUuid, err := createUuid()
	assert.NoError(t, err)

	// Test adding the first entry
	err = addImageHistory(chroot, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	verifyHistoryFile(t, 1, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)

	// Verify the config is unchanged
	currentConfigBytes, err := yaml.Marshal(config)
	assert.NoError(t, err, "failed to serialize current config")
	assert.Equal(t, originalConfigBytes, currentConfigBytes, "config should remain unchanged after adding image history")

	// Test adding another entry with a different uuid
	_, expectedUuid, err = createUuid()
	assert.NoError(t, err)
	err = addImageHistory(chroot, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	allHistory := verifyHistoryFile(t, 2, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)

	// Verify the imageUuid is unique for each entry
	assert.NotEqual(t, allHistory[0].ImageUuid, allHistory[1].ImageUuid, "imageUuid should be different for each entry")

}

func verifyHistoryFile(t *testing.T, expectedEntries int, expectedUuid string, expectedVersion string, expectedDate string, config imagecustomizerapi.Config, historyFilePath string) (allHistory []ImageHistory) {
	exists, err := file.PathExists(historyFilePath)
	assert.NoError(t, err, "error checking history file existence")
	assert.True(t, exists, "history file should exist")

	historyContent, err := os.ReadFile(historyFilePath)
	assert.NoError(t, err, "error reading history file")

	err = json.Unmarshal(historyContent, &allHistory)
	assert.NoError(t, err, "error unmarshalling history content")
	assert.Len(t, allHistory, expectedEntries, "history file should contain the expected number of entries")

	// Verify the last entry content
	entry := allHistory[expectedEntries-1]
	assert.Equal(t, expectedUuid, entry.ImageUuid, "imageUuid should match")
	assert.Equal(t, expectedVersion, entry.ToolVersion, "toolVersion should match")
	assert.Equal(t, expectedDate, entry.BuildTime, "buildTime should match")
	// Since the config is modified its entirety won't be an exact match; picking one consistent field to verify
	assert.Equal(t, config.OS.BootLoader.ResetType, entry.Config.OS.BootLoader.ResetType, "config bootloader reset type should match")

	verifyAdditionalFilesHashes(entry.Config.OS.AdditionalFiles, t)
	verifyAdditionalDirsHashes(entry.Config.OS.AdditionalDirs, t)
	verifyScriptsHashes(entry.Config.Scripts.PostCustomization, t)
	verifyScriptsHashes(entry.Config.Scripts.FinalizeCustomization, t)
	verifySshPublicKeysRedacted(entry.Config.OS.Users, t)

	return
}

func verifySshPublicKeysRedacted(users []imagecustomizerapi.User, t *testing.T) {
	for _, user := range users {
		for _, key := range user.SSHPublicKeys {
			assert.Equal(t, redactedString, key, "SSH public keys should be redacted")
		}
	}
}

func verifyScriptsHashes(scripts []imagecustomizerapi.Script, t *testing.T) {
	for _, script := range scripts {
		if script.Path != "" {
			verifyFileHash(t, script.Path, script.SHA256Hash)
		} else {
			assert.Empty(t, script.SHA256Hash, "script hash should be empty")
		}
	}
}
func verifyAdditionalFilesHashes(files imagecustomizerapi.AdditionalFileList, t *testing.T) {
	for _, f := range files {
		if f.Source != "" {
			verifyFileHash(t, f.Source, f.SHA256Hash)
		} else {
			assert.Empty(t, f.SHA256Hash, "SHA256Hash for additional files should be empty")
		}
	}
}

func verifyAdditionalDirsHashes(dirs imagecustomizerapi.DirConfigList, t *testing.T) {
	for _, dir := range dirs {
		assert.NotEmpty(t, dir.SHA256HashMap, "SHA256HashMap for additional directories should not be empty")
		for relPath, hash := range dir.SHA256HashMap {
			verifyFileHash(t, filepath.Join(dir.Source, relPath), hash)
		}
	}
}

func verifyFileHash(t *testing.T, path string, foundHash string) {
	assert.NotEmpty(t, foundHash, "SHA256Hash for file %s should not be empty", path)
	fullPath := filepath.Join(testDir, path)
	expectedHash, err := file.GenerateSHA256(fullPath)
	assert.NoError(t, err, "error generating SHA256 hash for file %s", path)
	assert.Equal(t, foundHash, expectedHash, "SHA256 hash for file %s should match", path)
}
