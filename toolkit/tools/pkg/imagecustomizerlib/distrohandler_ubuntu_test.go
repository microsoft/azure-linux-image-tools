// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageUbuntuUnsupportedAPIs(t *testing.T) {
	for _, baseImageInfo := range baseImageUbuntuAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageUbuntuUnsupportedAPIsHelper(t, baseImageInfo)
		})
	}
}

func testCustomizeImageUbuntuUnsupportedAPIsHelper(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageUbuntuUnsupportedAPIs_"+baseImageInfo.Name)
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
		OS: &imagecustomizerapi.OS{},
	}

	config.OS.BootLoader.ResetType = imagecustomizerapi.ResetBootLoaderTypeHard

	err := CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUbuntuUnsupportedBootloaderHardReset)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)

	config.OS.BootLoader.ResetType = imagecustomizerapi.ResetBootLoaderTypeDefault
	options.UseBaseImageRpmRepos = false

	err = CustomizeImage(t.Context(), testTmpDir, config, options)
	assert.ErrorIs(t, err, ErrUbuntuUnsupportedDisableBaseImageRepos)
	assert.ErrorIs(t, err, ErrUnsupportedDistroApi)
}
