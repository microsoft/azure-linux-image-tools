// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/systemd"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageServicesEnableDisable(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageServicesEnableDisable(t, baseImageInfo)
		})
	}
}

func testCustomizeImageServicesEnableDisable(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestServicesEnableDisable_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	configFile := filepath.Join(testDir, "services-config.yaml")
	err := CustomizeImageWithConfigFileOptions(t.Context(), configFile, ImageCustomizerOptions{
		BuildDir:             buildDir,
		InputImageFile:       baseImage,
		OutputImageFile:      outImageFilePath,
		OutputImageFormat:    "raw",
		UseBaseImageRpmRepos: true,
		PreviewFeatures:      baseImageInfo.PreviewFeatures,
	})
	if !assert.NoError(t, err) {
		return
	}

	// Connect to image.
	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true, /*includeDefaultMounts*/
		baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify state of services.
	consoleGettyEnabled, err := systemd.IsServiceEnabled("console-getty", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.True(t, consoleGettyEnabled)

	systemdPstoreEnabled, err := systemd.IsServiceEnabled("systemd-pstore", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.False(t, systemdPstoreEnabled)
}

func TestCustomizeImageServicesEnableUnknown(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageServicesEnableUnknown(t, baseImageInfo)
		})
	}
}

func testCustomizeImageServicesEnableUnknown(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestServicesEnableUnknown_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	config := imagecustomizerapi.Config{
		PreviewFeatures: baseImageInfo.PreviewFeatures,
		OS: &imagecustomizerapi.OS{
			Services: imagecustomizerapi.Services{
				Enable: []string{
					"chocolate-chip-muffin",
				},
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to enable service (service='chocolate-chip-muffin')")
	assert.ErrorContains(t, err, "chocolate-chip-muffin.service does not exist")
}

func TestCustomizeImageServicesDisableUnknown(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageServicesDisableUnknown(t, baseImageInfo)
		})
	}
}

func testCustomizeImageServicesDisableUnknown(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestServicesDisableUnknown_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	config := imagecustomizerapi.Config{
		PreviewFeatures: baseImageInfo.PreviewFeatures,
		OS: &imagecustomizerapi.OS{
			Services: imagecustomizerapi.Services{
				Disable: []string{
					"chocolate-chip-muffin",
				},
			},
		},
	}

	err := CustomizeImage(t.Context(), buildDir, testDir, &config, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "failed to disable service (service='chocolate-chip-muffin')")
}
