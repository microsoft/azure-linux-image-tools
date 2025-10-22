package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestBaseConfigsInputAndOutput(t *testing.T) {
	testTempDir := filepath.Join(tmpDir, "TestBaseConfigsInputAndOutput")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

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

func TestBaseConfigsInputAndOutput_FullRun(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTmpDir := filepath.Join(tmpDir, "TestBaseConfigsInputAndOutput_FullRun")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFile := filepath.Join(testTmpDir, "image.vhdx")

	currentConfigFile := filepath.Join(testDir, "hierarchical-config.yaml")

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, currentConfigFile, baseImage, nil,
		outImageFile, "vhdx", false, "")
	if !assert.NoError(t, err) {
		return
	}

	assert.FileExists(t, outImageFile)

}
