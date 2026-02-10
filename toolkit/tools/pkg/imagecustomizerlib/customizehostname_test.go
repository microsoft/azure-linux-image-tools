// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestUpdateHostname(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}

	// Setup environment.
	testTmpDir := filepath.Join(tmpDir, "TestUpdateHostname")
	defer os.RemoveAll(testTmpDir)

	proposedDir := testTmpDir
	chroot := safechroot.NewChroot(proposedDir, false)
	err := chroot.Initialize("", []string{}, []*safechroot.MountPoint{}, false)
	assert.NoError(t, err)
	defer chroot.Close(false)

	err = os.MkdirAll(filepath.Join(chroot.RootDir(), "etc"), os.ModePerm)
	assert.NoError(t, err)

	// Set hostname.
	expectedHostname := "testhostname"
	err = UpdateHostname(t.Context(), expectedHostname, chroot)
	assert.NoError(t, err)

	// Ensure hostname was correctly set.
	actualHostname, err := os.ReadFile(filepath.Join(chroot.RootDir(), "etc/hostname"))
	assert.NoError(t, err)
	assert.Equal(t, expectedHostname, string(actualHostname))
}

func TestCustomizeImageHostname(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageHostname(t, baseImageInfo)
		})
	}
}

func testCustomizeImageHostname(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestCustomizeImageHostname_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "hostname-config.yaml")
	outImageFilePath := filepath.Join(buildDir, "image.qcow2")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Ensure hostname was correctly set.
	actualHostname, err := os.ReadFile(filepath.Join(imageConnection.Chroot().RootDir(), "etc/hostname"))
	assert.NoError(t, err)
	assert.Equal(t, "testname", string(actualHostname))
}
