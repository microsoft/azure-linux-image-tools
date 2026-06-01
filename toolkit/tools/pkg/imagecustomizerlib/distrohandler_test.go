// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

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
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
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
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	options.InputImageFile = outImage1FilePath
	options.OutputImageFile = outImage2FilePath

	// Ensure 'distro-version' preview feature flag is enforced.
	configFile = filepath.Join(testDir, "nochange-config.yaml")
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrUnsupportedDistroVersion)

	// Enable 'distro-version' preview feature flag.
	configFile = filepath.Join(testDir, "distro-version-preview-feature.yaml")
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}
}
