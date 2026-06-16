// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/stretchr/testify/assert"
)

// Ensure unsupported-distro-version preview feature flag is required when distro version can't be parsed.
func TestCustomizeImageDistroVersionInvalid(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageDistroVersionInvalidHelper(t, "TestCustomizeImageDistroVersionInvalid"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageDistroVersionInvalidHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImage1FilePath := filepath.Join(testTempDir, "image1.qcow2")
	outImage2FilePath := filepath.Join(testTempDir, "image2.qcow2")
	configFile := filepath.Join(testDir, "distro-version-invalid.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
		InputImageFile:       baseImage,
		OutputImageFile:      outImage1FilePath,
		OutputImageFormat:    "qcow2",
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	// Corrupt the distro version.
	err := CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'unsupported-distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'unsupported-distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}
}

// Ensure unsupported-distro-version preview feature flag is required when distro version is too new (i.e. very large
// number).
func TestCustomizeImageDistroVersionNew(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageDistroVersionNewHelper(t, "TestCustomizeImageDistroVersionNew"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageDistroVersionNewHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTempDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImage1FilePath := filepath.Join(testTempDir, "image1.qcow2")
	outImage2FilePath := filepath.Join(testTempDir, "image2.qcow2")
	configFile := filepath.Join(testDir, "distro-version-new.yaml")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		UseBaseImageRpmRepos: true,
		InputImageFile:       baseImage,
		OutputImageFile:      outImage1FilePath,
		OutputImageFormat:    "qcow2",
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	// Set the distro version to a very large number.
	err := CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'unsupported-distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'unsupported-distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}
}

func TestAclValidateConfigPackageOpsRequireToolsDir(t *testing.T) {
	handler := newAclDistroHandler(targetos.TargetOsAzureContainerLinux3)

	rc := &ResolvedConfig{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureAzureContainerLinux,
		},
		ConfigChain: []*ConfigWithBasePath{
			{
				Config: &imagecustomizerapi.Config{
					OS: &imagecustomizerapi.OS{
						Packages: imagecustomizerapi.Packages{
							Install: []string{"vim"},
						},
					},
				},
			},
		},
	}

	err := handler.ValidateConfig(rc)
	assert.ErrorContains(t, err, "ACL package operations require --tools-dir")

	rc.Options.ToolsDir = "/some/tools/dir"
	err = handler.ValidateConfig(rc)
	assert.NoError(t, err)
}

func TestCustomizeImageUnsupportedPackageSnapshotTime(t *testing.T) {
	for _, baseImageInfo := range slices.Concat([]testBaseImageInfo{testBaseImageAzl4CoreEfi}, baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageUnsupportedPackageSnapshotTimeHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageUnsupportedPackageSnapshotTimeHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUnsupportedPackageSnapshotTime_"+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      "./out/image.vhdx",
		OutputImageFormat:    "vhdx",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	config := &imagecustomizerapi.Config{
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{imagecustomizerapi.PreviewFeaturePackageSnapshotTime},
		OS:              &imagecustomizerapi.OS{},
	}

	options.PackageSnapshotTime = "2025-01-01"

	err := CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedPackageSnapshotTime)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)

	options.PackageSnapshotTime = ""
	config.OS.Packages.SnapshotTime = "2025-01-01"

	err = CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedPackageSnapshotTime)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)
}

func TestCustomizeImageUnsupportedRpmSources(t *testing.T) {
	for _, baseImageInfo := range slices.Concat(baseImageUbuntuAll) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageUnsupportedRpmSourcesHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageUnsupportedRpmSourcesHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUnsupportedRpmSources_"+baseImageInfo.Name)
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")

	err := os.MkdirAll(testTmpDir, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	repoFile := filepath.Join(testTmpDir, "a.repo")
	err = os.WriteFile(repoFile, []byte{}, os.ModePerm)
	if !assert.NoError(t, err) {
		return
	}

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      "./out/image.vhdx",
		OutputImageFormat:    "vhdx",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	}

	config := &imagecustomizerapi.Config{
		OS: &imagecustomizerapi.OS{},
	}

	options.RpmsSources = []string{repoFile}

	err = CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUnsupportedRpmSources)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)
}
