// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/userutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageGroupsExistingGid(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGroupsExistingGid(t, baseImageInfo)
		})
	}
}

func testCustomizeImageGroupsExistingGid(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestGroupsExistingGid_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-root-gid.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "cannot set GID on a group that already exists (GID='42', group='root')")
}

func TestCustomizeImageGroupsNewGid(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGroupsNewGid(t, baseImageInfo)
		})
	}
}

func testCustomizeImageGroupsNewGid(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestGroupsNewGid_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-new-gid.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	groupEntry, err := userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "question")
	if assert.NoError(t, err) {
		assert.Equal(t, 99, groupEntry.GID)
	}

	passwdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "question")
	if assert.NoError(t, err) {
		assert.Equal(t, 99, passwdEntry.Uid)
		assert.Equal(t, 99, passwdEntry.Gid)
	}
}

func TestCustomizeImageGroupsNew(t *testing.T) {
	for _, baseImageInfo := range checkSkipForCustomizeDefaultImages(t) {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageGroupsNew(t, baseImageInfo)
		})
	}
}

func testCustomizeImageGroupsNew(t *testing.T, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, fmt.Sprintf("TestGroupsNew_%s", baseImageInfo.Name))
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-new.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false, baseImageInfo.MountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	groupEntry, err := userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "new-group")
	if !assert.NoError(t, err) {
		return
	}

	passwdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "new-user")
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, groupEntry.GID, passwdEntry.Gid)
}
