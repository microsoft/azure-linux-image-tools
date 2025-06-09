// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageMultiKernel(t *testing.T) {
	for _, version := range supportedAzureLinuxVersions {
		t.Run(string(version), func(t *testing.T) {
			testCustomizeImageMultiKernel(t, "TestCustomizeImageMultiKernel"+string(version),
				baseImageTypeCoreEfi, version)
		})
	}
}

func testCustomizeImageMultiKernel(t *testing.T, testName string, imageType baseImageType,
	imageVersion baseImageVersion,
) {
	baseImage := checkSkipForCustomizeImage(t, imageType, imageVersion)

	testTmpDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	configFile := ""
	switch imageVersion {
	case baseImageVersionAzl2:
		configFile = filepath.Join(testDir, "multikernel-azl2.yaml")

	case baseImageVersionAzl3:
		configFile = filepath.Join(testDir, "multikernel-azl3.yaml")
	}

	// Customize image.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check that the extraCommandLine was added to the grub.cfg file.
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")

	linuxCommandRegex := regexp.MustCompile(`linux.* console=tty0 console=ttyS0 `)
	matches := linuxCommandRegex.FindAllString(grubCfgContents, -1)

	switch imageVersion {
	case baseImageVersionAzl2:
		// AZL2's default grub.cfg file doesn't support multiple kernels.
		assert.GreaterOrEqual(t, len(matches), 1, "grub.cfg:\n%s", grubCfgContents)

	case baseImageVersionAzl3:
		// There should be multiple matching linux kernels, one for each installed kernel.
		assert.GreaterOrEqual(t, len(matches), 2, "grub.cfg:\n%s", grubCfgContents)
	}
}
