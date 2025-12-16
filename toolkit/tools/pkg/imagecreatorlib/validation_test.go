package imagecreatorlib

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func createDummyTarGz(filename string) error {
	// Create the output file
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Create a gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create a tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	content := []byte("dummy content")
	header := &tar.Header{
		Name: "dummy.txt",
		Mode: 0o600,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	return nil
}

func TestValidateOutput_AcceptsValidPaths(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	testTmpDir := filepath.Join(tmpDir, "TestValidateOutput_AcceptsValidPaths")
	defer os.RemoveAll(testTmpDir)

	buildDir := testTmpDir

	err = os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	baseConfigPath := testDir
	configFile := filepath.Join(testDir, "minimal-os.yaml")
	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	toolsFile := filepath.Join(testTmpDir, "tools.tar.gz")
	err = createDummyTarGz(toolsFile)
	assert.NoError(t, err)
	// just use the base config path as the RPM sources for this test
	// since we are not testing the RPM sources here.
	rpmSources := []string{baseConfigPath}

	outputImageDir := filepath.Join(buildDir, "out")
	err = os.MkdirAll(outputImageDir, os.ModePerm)
	assert.NoError(t, err)
	outputImageDirRelativeCwd, err := filepath.Rel(cwd, outputImageDir)
	assert.NoError(t, err)
	outputImageDirRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageDir)
	assert.NoError(t, err)

	outputImageFileNew := filepath.Join(outputImageDir, "new.vhdx")
	outputImageFileNewRelativeCwd, err := filepath.Rel(cwd, outputImageFileNew)
	assert.NoError(t, err)
	outputImageFileNewRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageFileNew)
	assert.NoError(t, err)

	outputImageFileExists := filepath.Join(outputImageDir, "exists.vhdx")
	err = file.Write("", outputImageFileExists)
	assert.NoError(t, err)
	outputImageFileExistsRelativeCwd, err := filepath.Rel(cwd, outputImageFileExists)
	assert.NoError(t, err)
	outputImageFileExistsRelativeConfig, err := filepath.Rel(baseConfigPath, outputImageFileExists)
	assert.NoError(t, err)

	outputImageFile := outputImageFileNew
	outputImageFormat := filepath.Ext(outputImageFile)[1:]
	packageSnapshotTime := ""

	// The output image file can be sepcified as an argument without being in specified the config.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNewRelativeCwd

	// The output image file can be specified as an argument relative to the current working directory.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	outputImageFile = outputImageDir

	// The output image file, specified as an argument, must not be a directory.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageDirRelativeCwd

	// The above is also true for relative paths.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageFileExists

	// The output image file, specified as an argument, may be a file that already exists.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	outputImageFile = outputImageFileExistsRelativeCwd

	// The above is also true for relative paths.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	outputImageFile = ""
	config.Output.Image.Path = outputImageFileNew

	// The output image file cab be specified in the config without being specified as an argument.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileNewRelativeConfig

	// The output image file can be specified in the config relative to the base config path.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file, specified in the config, must not be a directory.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageDirRelativeConfig

	// The above is also true for relative paths.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageFileExists

	// The output image file, specified in the config, may be a file that already exists.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileExistsRelativeConfig

	// The above is also true for relative paths.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNew
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified both as an argument and in the config.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file can even be invalid in the config if it is specified as an argument.
	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.NoError(t, err)
}

func TestValidateConfig_EmptyConfig(t *testing.T) {
	configFile := filepath.Join(testDir, "empty-config.yaml")
	config := &imagecustomizerapi.Config{}
	rpmSources := []string{}

	outputImageFile := ""
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""
	buildDir := "./"

	_, err := validateConfig(t.Context(), configFile, config, rpmSources, "", outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.ErrorContains(t, err, "storage.disks field is required in the config file")
}

func TestValidateConfig_EmptyPackagestoInstall(t *testing.T) {
	testTmpDir := filepath.Join(tmpDir, "TestValidateConfig_EmptyPackagestoInstall")
	defer os.RemoveAll(testTmpDir)

	err := os.MkdirAll(testTmpDir, os.ModePerm)
	assert.NoError(t, err)

	configFile := filepath.Join(testDir, "minimal-os.yaml")
	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	rpmSources := []string{testDir} // Use the test directory as a dummy RPM source
	outputImageFile := filepath.Join(testDir, "output.vhdx")
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""
	buildDir := "./"
	toolsFile := filepath.Join(testTmpDir, "tools.tar.gz")
	err = createDummyTarGz(toolsFile)
	assert.NoError(t, err)
	// Set the packages to install to an empty slice
	config.OS.Packages.Install = []string{}

	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, toolsFile, outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.ErrorContains(t, err, "no packages to install specified, please specify at least one package to install for a new image")
}

func TestValidateConfig_InvaliFieldsVerityConfig(t *testing.T) {
	configFile := filepath.Join(testDir, "../../imagecustomizerlib/testdata", "verity-config.yaml")

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	rpmSources := []string{testDir} // Use the test directory as a dummy RPM source
	outputImageFile := ""
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""
	buildDir := "./"

	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, "", outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.ErrorContains(t, err, "storage verity field is not supported by the image creator")
}

func TestValidateConfig_InvaliFieldsOverlaysConfig(t *testing.T) {
	configFile := filepath.Join(testDir, "../../imagecustomizerlib/testdata", "overlays-config.yaml")

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	rpmSources := []string{testDir} // Use the test directory as a dummy RPM source
	outputImageFile := ""
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""
	buildDir := "./"

	_, err = validateConfig(t.Context(), configFile, &config, rpmSources, "", outputImageFile, outputImageFormat,
		packageSnapshotTime, buildDir)
	assert.ErrorContains(t, err, "overlay field is not supported by the image creator")
}
