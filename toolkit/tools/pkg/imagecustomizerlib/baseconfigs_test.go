package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestBaseConfigIsValidNoPath(t *testing.T) {
	base := imagecustomizerapi.BaseConfig{
		Path: "",
	}
	err := base.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not be empty or whitespace")
}

func TestBaseConfigIsValidWhitespaces(t *testing.T) {
	base := imagecustomizerapi.BaseConfig{
		Path: "   ",
	}
	err := base.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "path must not be empty or whitespace")
}

func TestBaseConfigsInputAndOutput(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsInputAndOutput")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "current-config.yaml")

	options := ImageCustomizerOptions{
		BuildDir:       buildDir,
		InputImageFile: currentConfigFile,
	}

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(currentConfigFile, &config)
	assert.NoError(t, err)

	baseConfigPath, _ := filepath.Split(currentConfigFile)
	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	assert.NoError(t, err)

	rc := &ResolvedConfig{
		BaseConfigPath: absBaseConfigPath,
		Config:         &config,
		Options:        options,
	}

	err = resolveBaseConfigs(t.Context(), rc)
	assert.NoError(t, err)

	expectedInputPath := file.GetAbsPathWithBase(testDir, ".testimages/input-image-2.vhdx")
	expectedOutputPath := file.GetAbsPathWithBase(testDir, "./out/output-image-2.vhdx")
	expectedArtifactsPath := file.GetAbsPathWithBase(testDir, "./artifacts-2")

	assert.Equal(t, expectedInputPath, rc.Config.Input.Image.Path)
	assert.Equal(t, expectedOutputPath, rc.Config.Output.Image.Path)
	assert.Equal(t, expectedArtifactsPath, rc.Config.Output.Artifacts.Path)
	assert.Equal(t, "testname", rc.Config.OS.Hostname)

	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	actual := rc.Config.Output.Artifacts.Items
	assert.Equal(t, len(expectedItems), len(actual))

	for _, item := range expectedItems {
		assert.Containsf(t, actual, item, "expected output artifact item %q not found in resolved config: %v", item, actual)
	}
}

func TestBaseConfigsMalformed(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsMalformed")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "current-config-malformed.yaml")

	options := ImageCustomizerOptions{
		BuildDir:       buildDir,
		InputImageFile: currentConfigFile,
	}

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(currentConfigFile, &config)
	assert.NoError(t, err)

	baseConfigPath, _ := filepath.Split(currentConfigFile)
	absBaseConfigPath, err := filepath.Abs(baseConfigPath)
	assert.NoError(t, err)

	rc := &ResolvedConfig{
		BaseConfigPath: absBaseConfigPath,
		Config:         &config,
		Options:        options,
	}

	err = resolveBaseConfigs(t.Context(), rc)

	assert.ErrorContains(t, err, ErrInvalidBaseConfigs.Error())
}
