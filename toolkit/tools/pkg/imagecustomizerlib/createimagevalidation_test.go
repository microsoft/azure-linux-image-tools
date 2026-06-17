// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func TestValidateCreateImageOutput_AcceptsValidPaths(t *testing.T) {
	for _, vi := range []struct {
		name, configFile string
	}{
		{"azl3", "create-minimal-os.yaml"},
		{"azl4", fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testValidateCreateImageOutput_AcceptsValidPaths(t, vi.name, vi.configFile)
		})
	}
}

func testValidateCreateImageOutput_AcceptsValidPaths(t *testing.T, name string, configFileName string) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestValidateCreateImageOutput_AcceptsValidPaths_%s", name))
	defer os.RemoveAll(testTmpDir)

	buildDir := testTmpDir

	err = os.MkdirAll(buildDir, os.ModePerm)
	assert.NoError(t, err)

	baseConfigPath := testDir
	configFile := filepath.Join(testDir, configFileName)
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

	options := ImageCreateOptions{
		BuildDir:          buildDir,
		ToolsTar:          toolsFile,
		RpmsSources:       rpmSources,
		OutputImageFile:   outputImageFileNew,
		OutputImageFormat: imagecustomizerapi.ImageFormatType(filepath.Ext(outputImageFileNew)[1:]),
	}

	// The output image file can be specified as an argument without being in specified the config.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileNewRelativeCwd

	// The output image file can be specified as an argument relative to the current working directory.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageDir

	// The output image file, specified as an argument, must not be a directory.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	options.OutputImageFile = outputImageDirRelativeCwd

	// The above is also true for relative paths.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	options.OutputImageFile = outputImageFileExists

	// The output image file, specified as an argument, may be a file that already exists.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileExistsRelativeCwd

	// The above is also true for relative paths.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	options.OutputImageFile = ""
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified in the config without being specified as an argument.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileNewRelativeConfig

	// The output image file can be specified in the config relative to the base config path.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file, specified in the config, must not be a directory.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageDirRelativeConfig

	// The above is also true for relative paths.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is a directory")

	config.Output.Image.Path = outputImageFileExists

	// The output image file, specified in the config, may be a file that already exists.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageFileExistsRelativeConfig

	// The above is also true for relative paths.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	options.OutputImageFile = outputImageFileNew
	config.Output.Image.Path = outputImageFileNew

	// The output image file can be specified both as an argument and in the config.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)

	config.Output.Image.Path = outputImageDir

	// The output image file can even be invalid in the config if it is specified as an argument.
	_, err = validateCreateImageConfig(t.Context(), baseConfigPath, &config, options)
	assert.NoError(t, err)
}

func TestValidateCreateImageConfig_EmptyConfig(t *testing.T) {
	baseConfigPath := testDir
	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeatureCreate},
	}

	options := ImageCreateOptions{
		OutputImageFormat: "vhdx",
		BuildDir:          "./",
	}

	_, err := validateCreateImageConfig(t.Context(), baseConfigPath, config, options)
	assert.ErrorContains(t, err, "storage.disks field is required in the config file")
}

func TestValidateCreateImageConfig_EmptyPackagestoInstall(t *testing.T) {
	for _, vi := range []struct {
		name, configFile string
	}{
		{"azl3", "create-minimal-os.yaml"},
		{"azl4", fmt.Sprintf("create-azl4-%s.yaml", runtime.GOARCH)},
	} {
		t.Run(vi.name, func(t *testing.T) {
			testValidateCreateImageConfig_EmptyPackagestoInstall(t, vi.name, vi.configFile)
		})
	}
}

func testValidateCreateImageConfig_EmptyPackagestoInstall(t *testing.T, name string, configFileName string) {
	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestValidateCreateImageConfig_EmptyPackagestoInstall_%s", name))
	defer os.RemoveAll(testTmpDir)

	err := os.MkdirAll(testTmpDir, os.ModePerm)
	assert.NoError(t, err)

	configFile := filepath.Join(testDir, configFileName)
	var config imagecustomizerapi.Config
	err = imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	options := ImageCreateOptions{
		RpmsSources:       []string{testDir}, // Use the test directory as a dummy RPM source
		OutputImageFile:   filepath.Join(testDir, "output.vhdx"),
		OutputImageFormat: "vhdx",
		BuildDir:          "./",
		ToolsTar:          filepath.Join(testTmpDir, "tools.tar.gz"),
	}

	err = createDummyTarGz(options.ToolsTar)
	assert.NoError(t, err)
	// Set the packages to install to an empty slice
	config.OS.Packages.Install = []string{}
	config.OS.Packages.InstallLists = []string{}
	_, err = validateCreateImageConfig(t.Context(), testDir, &config, options)
	assert.ErrorContains(t, err, "no packages to install specified, please specify at least one package to install for a new image")
}

func TestValidateCreateImageConfig_InvalidFieldsVerityConfig(t *testing.T) {
	configFile := filepath.Join(testDir, "verity-config.yaml")

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	options := ImageCreateOptions{
		RpmsSources:       []string{testDir}, // Use the test directory as a dummy RPM source
		OutputImageFormat: "vhdx",
		BuildDir:          "./",
	}

	_, err = validateCreateImageConfig(t.Context(), testDir, &config, options)
	assert.ErrorContains(t, err, "storage verity field is not supported by the create subcommand")
}

func TestValidateCreateImageConfig_InvalidFieldsOverlaysConfig(t *testing.T) {
	configFile := filepath.Join(testDir, "overlays-config.yaml")

	var config imagecustomizerapi.Config
	err := imagecustomizerapi.UnmarshalYamlFile(configFile, &config)
	assert.NoError(t, err)

	options := ImageCreateOptions{
		RpmsSources:       []string{testDir}, // Use the test directory as a dummy RPM source
		OutputImageFormat: "vhdx",
		BuildDir:          "./",
	}

	_, err = validateCreateImageConfig(t.Context(), testDir, &config, options)
	assert.ErrorContains(t, err, "overlay field is not supported by the create subcommand")
}

// An empty --tools-file path is allowed; the create path falls back to
// auto-provisioning the tools chroot via OCI pull.
func TestValidateToolsTarFile_EmptyAllowed(t *testing.T) {
	assert.NoError(t, validateToolsTarFile(""))
}

// When --tools-file IS supplied, the on-disk validation (existence + tar.gz
// structure) must still run.
func TestValidateToolsTarFile_MissingPath(t *testing.T) {
	err := validateToolsTarFile("/definitely/not/a/real/path/tools.tar.gz")
	assert.ErrorContains(t, err, "does not exist")
}
