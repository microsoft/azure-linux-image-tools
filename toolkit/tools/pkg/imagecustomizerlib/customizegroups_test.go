// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/userutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageGroupsExistingGid(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageGroupExistingGid")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-root-gid.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "cannot set GID on a group that already exists (GID='42', group='root')")
}

func TestCustomizeImageGroupsNewGid(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageGroupNewGid")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-new-gid.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	groupEntry, err := userutils.GetGroupEntry(imageConnection.Chroot().RootDir(), "question")
	if assert.NoError(t, err) {
		assert.Equal(t, 42, groupEntry.GID)
	}

	passwdEntry, err := userutils.GetPasswdFileEntryForUser(imageConnection.Chroot().RootDir(), "question")
	if assert.NoError(t, err) {
		assert.Equal(t, 42, passwdEntry.Uid)
		assert.Equal(t, 42, passwdEntry.Gid)
	}
}

func TestCustomizeImageGroupsNew(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageGroupNew")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")
	configFile := filepath.Join(testDir, "user-group-new.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
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
