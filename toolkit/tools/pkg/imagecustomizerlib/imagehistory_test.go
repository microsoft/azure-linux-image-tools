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
	expectedVersion := "0.1.0"
	expectedDate := time.Now().Format("2006-01-02T15:04:05Z")
	_, expectedUuid, err := createUuid()
	assert.NoError(t, err)

	// Test adding the first entry
	err = addImageHistory(chroot, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	verifyHistoryFile(t, 1, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)

	// Test adding another entry with a different uuid
	_, expectedUuid, err = createUuid()
	assert.NoError(t, err)
	err = addImageHistory(chroot, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	verifyHistoryFile(t, 2, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)
}

func verifyHistoryFile(t *testing.T, expectedEntries int, expectedUuid string, expectedVersion string, expectedDate string, config imagecustomizerapi.Config, historyFilePath string) {
	exists, err := file.PathExists(historyFilePath)
	assert.NoError(t, err, "error checking history file existence")
	assert.True(t, exists, "history file should exist")

	historyContent, err := os.ReadFile(historyFilePath)
	assert.NoError(t, err, "error reading history file")

	var allHistory []ImageHistory
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

	// Verify the imageUuid is unique for each entry
	if expectedEntries == 2 {
		assert.NotEqual(t, allHistory[0].ImageUuid, allHistory[1].ImageUuid, "imageUuid should be different for each entry")
	}
}

func TestDeepCopyConfig(t *testing.T) {
	config := createTestConfig("imagehistory-config.yaml", t)

	copiedConfig, err := deepCopyConfig(&config)

	assert.NoError(t, err, "deepCopyConfig should not return an error")
	assert.NotSame(t, config, copiedConfig, "deepCopyConfig should create a new instance")
}

func TestModifyConfig(t *testing.T) {
	config := createTestConfig("imagehistory-config.yaml", t)

	err := modifyConfig(&config, testDir)
	assert.NoError(t, err, "modifyConfig should not return an error")

	isNotEmptyAdditionalFilesHashes(config.OS.AdditionalFiles, t)

	isNotEmptyAdditionalDirsHashes(config.OS.AdditionalDirs, t)

	isNotEmptyScriptsHashes(config.Scripts.PostCustomization, t)
	isNotEmptyScriptsHashes(config.Scripts.FinalizeCustomization, t)

	verifySshPublicKeysRedacted(config.OS.Users, t)
}

func TestRedactSshPublicKeys(t *testing.T) {
	mockUsers := []imagecustomizerapi.User{
		{
			SSHPublicKeys: []string{"key1", "key2"},
		},
	}

	err := redactSshPublicKeys(mockUsers)
	assert.NoError(t, err, "redactSshPublicKeys should not return an error")

	verifySshPublicKeysRedacted(mockUsers, t)
}

func TestPopulateAdditionalDirs(t *testing.T) {
	dirs := createTestConfig("adddirs-config.yaml", t).OS.AdditionalDirs

	err := populateAdditionalDirs(dirs, testDir)
	assert.NoError(t, err, "populateAdditionalDirs should not return an error")

	isNotEmptyAdditionalDirsHashes(dirs, t)
}

func TestPopulateAdditionalFiles(t *testing.T) {
	files := createTestConfig("addfiles-config.yaml", t).OS.AdditionalFiles

	err := populateAdditionalFiles(files, testDir)
	assert.NoError(t, err, "populateAdditionalFiles should not return an error")

	isNotEmptyAdditionalFilesHashes(files, t)
}

func TestPopulateScriptsList(t *testing.T) {
	scripts := createTestConfig("runscripts-config.yaml", t).Scripts

	err := populateScriptsList(scripts, testDir)
	assert.NoError(t, err, "populateScriptsList should not return an error")

	isNotEmptyScriptsHashes(scripts.PostCustomization, t)
	isNotEmptyScriptsHashes(scripts.FinalizeCustomization, t)
}

func verifySshPublicKeysRedacted(users []imagecustomizerapi.User, t *testing.T) {
	for _, user := range users {
		for _, key := range user.SSHPublicKeys {
			assert.Equal(t, redactedString, key, "SSH public keys should be redacted")
		}
	}
}

func isNotEmptyScriptsHashes(scripts []imagecustomizerapi.Script, t *testing.T) {
	for _, script := range scripts {
		if script.Path != "" {
			assert.NotEmpty(t, script.SHA256Hash, "script hash should not be empty")
		} else {
			assert.Empty(t, script.SHA256Hash, "script hash should be empty")
		}
	}
}
func isNotEmptyAdditionalFilesHashes(files imagecustomizerapi.AdditionalFileList, t *testing.T) {
	for _, file := range files {
		if file.Source != "" {
			assert.NotEmpty(t, file.SHA256Hash, "SHA256Hash for additional files should not be empty")
		} else {
			assert.Empty(t, file.SHA256Hash, "SHA256Hash for additional files should be empty")
		}
	}
}

func isNotEmptyAdditionalDirsHashes(dirs imagecustomizerapi.DirConfigList, t *testing.T) {
	for _, dir := range dirs {
		assert.NotEmpty(t, dir.SHA256HashMap, "SHA256HashMap for additional directories should not be empty")
	}
}
