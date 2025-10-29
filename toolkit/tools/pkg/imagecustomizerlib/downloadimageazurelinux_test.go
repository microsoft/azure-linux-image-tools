// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAzureLinuxOciUriAZL2Latest(t *testing.T) {
	ociImage, err := generateAzureLinuxOciUri(imagecustomizerapi.AzureLinuxImage{
		Variant: "minimal-os",
		Version: "2.0",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mcr.microsoft.com/azurelinux/2.0/image/minimal-os:latest", ociImage.Uri)
}

func TestGenerateAzureLinuxOciUriAZL3Latest(t *testing.T) {
	ociImage, err := generateAzureLinuxOciUri(imagecustomizerapi.AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3.0",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest", ociImage.Uri)
}

func TestGenerateAzureLinuxOciUriAZL2Date(t *testing.T) {
	ociImage, err := generateAzureLinuxOciUri(imagecustomizerapi.AzureLinuxImage{
		Variant: "minimal-os",
		Version: "2.0.20240425",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mcr.microsoft.com/azurelinux/2.0/image/minimal-os:2.0.20240425", ociImage.Uri)
}

func TestGenerateAzureLinuxOciUriAZL3Date(t *testing.T) {
	ociImage, err := generateAzureLinuxOciUri(imagecustomizerapi.AzureLinuxImage{
		Variant: "minimal-os",
		Version: "3.0.20250910",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mcr.microsoft.com/azurelinux/3.0/image/minimal-os:3.0.20250910", ociImage.Uri)
}

func TestParseInputImageAZLValid(t *testing.T) {
	inputImage, err := parseInputImage("azurelinux:minimal-os:3.0")
	assert.NoError(t, err)
	if !assert.NotNil(t, inputImage.AzureLinux) {
		assert.Equal(t, "minimal-os", inputImage.AzureLinux.Variant)
		assert.Equal(t, "3.0", inputImage.AzureLinux.Version)
	}
}

func TestParseInputImageAZLMissingVersion(t *testing.T) {
	_, err := parseInputImage("azurelinux:minimal-os")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "missing Azure Linux version value")
}

func TestParseInputImageAZLEmptyVersion(t *testing.T) {
	_, err := parseInputImage("azurelinux:minimal-os:")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "invalid 'version' field")
}

func TestParseInputImageAZLEmptyVariant(t *testing.T) {
	_, err := parseInputImage("azurelinux::3.0")
	assert.ErrorIs(t, err, ErrInvalidInputImageStringFormat)
	assert.ErrorContains(t, err, "invalid 'variant' field")
}

func TestCustomizeImageAZLBaseImageConfigValid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageAZLBaseImageConfigValid")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "azurelinux-base-image.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	options := ImageCustomizerOptions{
		BuildDir:             buildDir,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		ImageCacheDir:        sharedImageCacheDir,
	}

	// Customize image.
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
	assert.Equal(t, "echidna", string(actualHostname))
}

func TestCustomizeImageAZLBaseImageCliValid(t *testing.T) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageAZLBaseImageCliValid")
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
		InputImage:           "azurelinux:minimal-os:3.0",
	}

	// Customize image.
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
