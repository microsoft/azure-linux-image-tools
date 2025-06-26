package imagecreatorlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestValidateOutput_AcceptsValidPaths(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	buildDir := filepath.Join(tmpDir, "TestValidateOutput_AcceptsValidPaths")
	err = os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	baseConfigPath := testDir
	configFile := filepath.Join(testDir, "minimal-os.yaml")
	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
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
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNewRelativeCwd

	// The output image file can be specified as an argument relative to the current working directory.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageDir

	// The output image file, specified as an argument, must not be a directory.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageDirRelativeCwd

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	outputImageFile = outputImageFileExists

	// The output image file, specified as an argument, may be a file that already exists.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileExistsRelativeCwd

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = ""
	config.Output.Image.Path = outputImageFileNew

	// The output image file cab be specified in the config without being specified as an argument.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileNewRelativeConfig

	// The output image file can be specified in the config relative to the base config path.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file, specified in the config, must not be a directory.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageDirRelativeConfig

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageFileExists

	// The output image file, specified in the config, may be a file that already exists.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileExistsRelativeConfig

	// The above is also true for relative paths.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	outputImageFile = outputImageFileNew
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified both as an argument and in the config.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file can even be invalid in the config if it is specified as an argument.
	err = validateConfig(baseConfigPath, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.NoError(t, err)
}

func TestValidateConfig_EmptyConfig(t *testing.T) {
	baseConfigPath := testDir
	config := &imagecustomizerapi.Config{}
	rpmSources := []string{}

	outputImageFile := ""
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""

	err := validateConfig(baseConfigPath, config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.ErrorContains(t, err, "storage.disks field is required in the config file")
}

func TestValidateConfig_EmptyPackagestoInstall(t *testing.T) {
	configFile := filepath.Join(testDir, "minimal-os.yaml")
	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	rpmSources := []string{testDir} // Use the test directory as a dummy RPM source
	outputImageFile := ""
	outputImageFormat := "vhdx"
	packageSnapshotTime := ""
	// Set the packages to install to an empty slice
	config.OS.Packages.Install = []string{}
	err = validateConfig(configFile, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
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

	err = validateConfig(configFile, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
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

	err = validateConfig(configFile, &config, rpmSources, outputImageFile, outputImageFormat,
		packageSnapshotTime)
	assert.ErrorContains(t, err, "overlay field is not supported by the image creator")
}
