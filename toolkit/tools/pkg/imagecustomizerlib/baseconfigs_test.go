package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/systemd"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/userutils"
	"github.com/stretchr/testify/assert"
)

func TestBaseConfigsInputAndOutput(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsInputAndOutput")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
	}

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(currentConfigFile, &config)
	assert.NoError(t, err)

	rc, err := ValidateConfig(t.Context(), testDir, &config, false, options)
	assert.NoError(t, err)

	// Verify resolved values
	expectedInputPath := file.GetAbsPathWithBase(testDir, "testimages/empty.vhdx")
	expectedOutputPath := file.GetAbsPathWithBase(testDir, "./out/output-image-2.vhdx")
	expectedArtifactsPath := file.GetAbsPathWithBase(testDir, "./artifacts-2")

	assert.Equal(t, expectedInputPath, rc.InputImageFile)
	assert.Equal(t, expectedOutputPath, rc.OutputImageFile)
	assert.Equal(t, expectedArtifactsPath, rc.OutputArtifacts.Path)
	assert.Equal(t, "testname", rc.Config.OS.Hostname)

	// Verify merged artifact items
	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	actual := rc.OutputArtifacts.Items
	assert.Equal(t, len(expectedItems), len(actual))

	assert.ElementsMatch(t, expectedItems, actual,
		"output artifact items should match - expected: %v, got: %v", expectedItems, actual)
}

func TestBaseConfigsFullRun(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTmpDir := filepath.Join(tmpDir, "TestBaseConfigsFullRun")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.vhdx")

	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, currentConfigFile, baseImage, nil,
		outImageFilePath, "raw", true, "")
	if !assert.NoError(t, err) {
		return
	}

	assert.FileExists(t, outImageFilePath)

	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	hostnamePath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/hostname")
	data, err := os.ReadFile(hostnamePath)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "testname", data)

	// Verify users
	baseadminEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test-user-base")
	if assert.NoError(t, err) {
		assert.Contains(t, baseadminEntry.HomeDirectory, "test-base")
	}

	appuserEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "test-user")
	if assert.NoError(t, err) {
		assert.Contains(t, appuserEntry.HomeDirectory, "test")
	}

	// Verify groups
	_, err = userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "test-group-base")
	assert.NoError(t, err)

	_, err = userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "test-group")
	assert.NoError(t, err)

	// Verify additional files
	aFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "mnt/a/a.txt")
	bFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "mnt/b/b.txt")

	// Assert files exist
	_, err = os.Stat(aFilePath)
	assert.NoError(t, err, "expected a.txt to exist at %s", aFilePath)
	_, err = os.Stat(bFilePath)
	assert.NoError(t, err, "expected b.txt to exist at %s", bFilePath)

	// Verify services
	sshdEnabled, err := systemd.IsServiceEnabled("sshd", imageConnection.Chroot())
	if err != nil {
		t.Fatalf("failed to check sshd: %v", err)
	}
	assert.True(t, sshdEnabled, "expected sshd to be enabled")

	chronydEnabled, err := systemd.IsServiceEnabled("chronyd", imageConnection.Chroot())
	if err != nil {
		t.Fatalf("failed to check chronyd: %v", err)
	}
	assert.False(t, chronydEnabled, "expected chronyd to be disabled")
}
