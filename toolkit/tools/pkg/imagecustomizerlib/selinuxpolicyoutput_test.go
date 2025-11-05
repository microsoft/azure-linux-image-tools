// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestOutputSelinuxPolicy(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestOutputSelinuxPolicy")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	originalConfigFile := filepath.Join(testDir, "selinux-policy-output.yaml")
	configFile := filepath.Join(testTempDir, "selinux-policy-output.yaml")
	outputSelinuxPolicyDir := filepath.Join(testTempDir, "selinux-output")

	err := file.Copy(originalConfigFile, configFile)
	assert.NoError(t, err)

	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	verifyExtractedSelinuxPolicy(t, outputSelinuxPolicyDir)
}

func verifyExtractedSelinuxPolicy(t *testing.T, outputDir string) {
	targetedDir := filepath.Join(outputDir, "targeted")
	exists, err := file.DirExists(targetedDir)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected 'targeted' directory to exist in output: %s", targetedDir)

	expectedPaths := []string{
		"seusers",
		"contexts",
		"policy",
	}

	for _, expectedPath := range expectedPaths {
		fullPath := filepath.Join(targetedDir, expectedPath)
		exists, err := file.PathExists(fullPath)
		assert.NoError(t, err)
		assert.True(t, exists, "Expected SELinux path to exist: %s", expectedPath)
	}

	t.Logf("Successfully verified SELinux policy extraction")
}
