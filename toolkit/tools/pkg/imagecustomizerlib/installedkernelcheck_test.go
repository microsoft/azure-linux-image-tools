// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageMissingKernel(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageMissingKernel")
	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, "no-kernel-config.yaml")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "no installed kernel found")
}
