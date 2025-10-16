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
		BuildDir: buildDir,
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
	assert.Equal(t, expectedArtifactsPath, rc.Config.Output.Artifacts.Path)
	assert.Equal(t, "testname", rc.Config.OS.Hostname)

	// Verify merged artifact items
	expectedItems := []imagecustomizerapi.OutputArtifactsItemType{
		imagecustomizerapi.OutputArtifactsItemUkis,
		imagecustomizerapi.OutputArtifactsItemShim,
	}
	actual := rc.Config.Output.Artifacts.Items
	assert.Equal(t, len(expectedItems), len(actual))

	for _, item := range expectedItems {
		assert.Containsf(t, actual, item, "expected output artifact item %q not found in resolved config: %v",
			item, actual)
	}
}

func TestBaseConfigsMalformed(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsMalformed")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "current-config-malformed.yaml")

	options := ImageCustomizerOptions{
		BuildDir: buildDir,
	}

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(currentConfigFile, &config)
	assert.NoError(t, err)

	_, err = ValidateConfig(t.Context(), testDir, &config, false, options)

	assert.ErrorContains(t, err, ErrInvalidBaseConfigs.Error())
}
