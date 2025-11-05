// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageOciBaseImageInvalid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageOciBaseImageInvalid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "oci-base-image.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
	}

	// No image cache directory.
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrOciDownloadMissingCacheDir)

	// Image cache directory points to a file.
	options.ImageCacheDir = configFile
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrOciDownloadCreateCacheDir)
}

func TestCustomizeImageOciBaseImageValid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageOciBaseImageValid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "oci-base-image.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	imageCacheDir := filepath.Join(testTmpDir, "image-cache")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		ImageCacheDir:        imageCacheDir,
	}

	logMessagesHook := logMessagesHook.AddSubHook()
	defer logMessagesHook.Close()

	// Customize image, with empty cache.
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	expectedDownloadLogMessage := logger.MemoryLogMessage{
		Message: "Downloading OCI file (image.vhdx)",
		Level:   logrus.DebugLevel,
	}

	logMessages := logMessagesHook.ConsumeMessages()
	assert.Contains(t, logMessages, expectedDownloadLogMessage)

	// Customize image, with populated cache.
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	logMessages = logMessagesHook.ConsumeMessages()
	assert.NotContains(t, logMessages, expectedDownloadLogMessage)

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure hostname was correctly set.
	actualHostname, err := os.ReadFile(filepath.Join(imageConnection.Chroot().RootDir(), "etc/hostname"))
	assert.NoError(t, err)
	assert.Equal(t, "sugarglider", string(actualHostname))
}

func TestCustomizeImageOciBaseImageCliInvalid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageOciBaseImageCliInvalid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "oci-preview-feature.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	imageCacheDir := filepath.Join(testTmpDir, "image-cache")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		ImageCacheDir:        imageCacheDir,
		InputImage:           "bird:magpie",
	}

	// Bad string format
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)

	// Bad URI
	options.InputImage = "oci:mcr.microsoft.com"
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)

	// No image cache directory.
	options.ImageCacheDir = ""
	options.InputImage = "oci:mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest"
	err = CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	assert.ErrorIs(t, err, ErrOciDownloadMissingCacheDir)
}

func TestCustomizeImageOciBaseImageCliValid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageOciBaseImageCliValid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "oci-preview-feature.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		ImageCacheDir:        sharedImageCacheDir,
		InputImage:           "oci:mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest",
	}

	// Customize image, with empty cache.
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, options)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure hostname was correctly set.
	actualHostname, err := os.ReadFile(filepath.Join(imageConnection.Chroot().RootDir(), "etc/hostname"))
	assert.NoError(t, err)
	assert.Equal(t, "blue-tongued-skink", string(actualHostname))
}
