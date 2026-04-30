// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageMissingKernel(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageMissingKernel")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	configFile := filepath.Join(testDir, missingKernelConfigFile(t, baseImageInfo))
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	assert.ErrorContains(t, err, "no installed kernel found")
}

// missingKernelConfigFile returns the no-kernel-config test config file appropriate for the
// given base image version. Azure Linux 4.0 splits the kernel into a `kernel` meta-package
// (no files) and a `kernel-core` package, so removing only `kernel` is a no-op for the
// installed-kernel check. AzL2/AzL3 ship a single monolithic `kernel` package.
func missingKernelConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "no-kernel-config.yaml"
	case baseImageVersionAzl4:
		return "no-kernel-config-azl4.yaml"
	default:
		t.Fatalf("unsupported base image version for missing-kernel test: %s", baseImageInfo.Version)
		return ""
	}
}
