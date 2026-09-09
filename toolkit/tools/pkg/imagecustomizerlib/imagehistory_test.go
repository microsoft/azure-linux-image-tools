package imagecustomizerlib

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"
)

func createTestConfig(configFilePath string, t *testing.T) imagecustomizerapi.Config {
	configFile := filepath.Join(testDir, configFilePath)

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalAndValidateYamlFile(configFile, &config)
	assert.NoError(t, err)
	return config
}

func TestAddImageHistory(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testAddImageHistory(t, baseImageInfo)
		})
	}
}

func testAddImageHistory(t *testing.T, baseImageInfo testBaseImageInfo) {
	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestAddImageHistory_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	historyDir := filepath.Join(testTmpDir, customizerLoggingDir)
	historyFilePath := filepath.Join(historyDir, historyFileName)
	config := createTestConfig("imagehistory-config.yaml", t)
	// Serialize the config before calling addImageHistory
	originalConfigBytes, err := yaml.Marshal(config)
	assert.NoError(t, err, "failed to serialize original config")

	expectedVersion := "0.1.0"
	expectedDate := time.Now().Format(buildTimeFormat)
	_, expectedUuid, err := randomization.CreateUuid()
	assert.NoError(t, err)

	// Test adding the first entry
	err = addImageHistoryHelper(t.Context(), testTmpDir, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	verifyHistoryFile(t, 1, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)

	// Verify the config is unchanged
	currentConfigBytes, err := yaml.Marshal(config)
	assert.NoError(t, err, "failed to serialize current config")
	assert.Equal(t, originalConfigBytes, currentConfigBytes, "config should remain unchanged after adding image history")

	// Test adding another entry with a different uuid
	_, expectedUuid, err = randomization.CreateUuid()
	assert.NoError(t, err)
	err = addImageHistoryHelper(t.Context(), testTmpDir, expectedUuid, testDir, expectedVersion, expectedDate, &config)
	assert.NoError(t, err, "addImageHistory should not return an error")

	allHistory := verifyHistoryFile(t, 2, expectedUuid, expectedVersion, expectedDate, config, historyFilePath)

	// Verify the imageUuid is unique for each entry
	assert.NotEqual(t, allHistory[0].ImageUuid, allHistory[1].ImageUuid, "imageUuid should be different for each entry")
}

func verifyImageHistoryFile(t *testing.T, expectedEntries int, config imagecustomizerapi.Config, rootPath string,
) (allHistory []ImageHistory) {
	return verifyHistoryFile(t, expectedEntries, "", ToolVersion, "", config,
		filepath.Join(rootPath, customizerLoggingDir, historyFileName))
}

func verifyHistoryFile(t *testing.T, expectedEntries int, expectedUuid string, expectedVersion string,
	expectedDate string, config imagecustomizerapi.Config, historyFilePath string,
) (allHistory []ImageHistory) {
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
	if expectedUuid != "" {
		assert.Equal(t, expectedUuid, entry.ImageUuid, "imageUuid should match")
	}
	assert.Equal(t, expectedVersion, entry.ToolVersion, "toolVersion should match")
	if expectedDate != "" {
		assert.Equal(t, expectedDate, entry.BuildTime, "buildTime should match")
	}
	// Since the config is modified its entirety won't be an exact match; picking one consistent field to verify
	assert.Equal(t, config.OS.BootLoader.ResetType, entry.Config.OS.BootLoader.ResetType, "config bootloader reset type should match")

	verifyAdditionalFilesHashes(t, entry.Config.OS.AdditionalFiles)
	verifyAdditionalDirsHashes(t, entry.Config.OS.AdditionalDirs)
	verifyScriptsHashes(t, entry.Config.Scripts.PostCustomization)
	verifyScriptsHashes(t, entry.Config.Scripts.FinalizeCustomization)
	verifySshPublicKeysRedacted(t, entry.Config.OS.Users)

	return
}

func verifySshPublicKeysRedacted(t *testing.T, users []imagecustomizerapi.User) {
	for _, user := range users {
		for _, key := range user.SSHPublicKeys {
			assert.Equal(t, redactedString, key, "SSH public keys should be redacted")
		}
	}
}

func verifyScriptsHashes(t *testing.T, scripts []imagecustomizerapi.Script) {
	for _, script := range scripts {
		if script.Path != "" {
			verifyFileHash(t, script.Path, script.SHA256Hash)
		} else {
			assert.Empty(t, script.SHA256Hash, "script hash should be empty")
		}
	}
}

func verifyAdditionalFilesHashes(t *testing.T, files imagecustomizerapi.AdditionalFileList) {
	for _, f := range files {
		if f.Source != "" {
			verifyFileHash(t, f.Source, f.SHA256Hash)
		} else {
			assert.Empty(t, f.SHA256Hash, "SHA256Hash for additional files should be empty")
		}
	}
}

func verifyAdditionalDirsHashes(t *testing.T, dirs imagecustomizerapi.DirConfigList) {
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

// TestPopulateAdditionalDirsSymlink verifies Bug 22991: history hashing must not
// follow symlinks. It records the SHA256 of the link *target string* so that a
// dangling link (or a link to a special file like /dev/zero) does not error out
// or read unbounded data.
func TestPopulateAdditionalDirsSymlink(t *testing.T) {
	baseConfigPath := t.TempDir()
	srcDir := filepath.Join(baseConfigPath, "src")
	assert.NoError(t, os.MkdirAll(srcDir, 0o755))

	assert.NoError(t, os.WriteFile(filepath.Join(srcDir, "target.txt"), []byte("hello"), 0o644))
	assert.NoError(t, os.Symlink("target.txt", filepath.Join(srcDir, "link.txt")))
	assert.NoError(t, os.Symlink("/does/not/exist/on/host", filepath.Join(srcDir, "dangling.txt")))

	dirs := imagecustomizerapi.DirConfigList{{Source: "src", Destination: "/"}}
	err := populateAdditionalDirs(dirs, baseConfigPath)
	assert.NoError(t, err) // previously failed on the dangling symlink

	hashes := dirs[0].SHA256HashMap
	assert.Len(t, hashes, 3)

	// Regular file: hash of its contents.
	fileHash, err := file.GenerateSHA256(filepath.Join(srcDir, "target.txt"))
	assert.NoError(t, err)
	assert.Equal(t, fileHash, hashes["target.txt"])

	// Symlinks: hash of the target string, not the dereferenced contents.
	assert.Equal(t, sha256HexOfString("target.txt"), hashes["link.txt"])
	assert.Equal(t, sha256HexOfString("/does/not/exist/on/host"), hashes["dangling.txt"])
}

func sha256HexOfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
