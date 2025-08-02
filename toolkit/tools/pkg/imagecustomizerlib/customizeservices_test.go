// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/systemd"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageServicesEnableDisable(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageServicesEnableDisable")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	configFile := filepath.Join(testDir, "services-config.yaml")
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to image.
	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, true /*includeDefaultMounts*/, coreEfiMountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify state of services.
	consoleGettyEnabled, err := systemd.IsServiceEnabled("console-getty", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.True(t, consoleGettyEnabled)

	chronydEnabled, err := systemd.IsServiceEnabled("chronyd", imageConnection.Chroot())
	assert.NoError(t, err)
	assert.False(t, chronydEnabled)
}

func TestCustomizeImageServicesEnableUnknown(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageServicesEnableUnknown")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	config := imagecustomizerapi.Config{
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
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageServicesDisableUnknown")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	config := imagecustomizerapi.Config{
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
	assert.ErrorContains(t, err, "No such file or directory")
}
