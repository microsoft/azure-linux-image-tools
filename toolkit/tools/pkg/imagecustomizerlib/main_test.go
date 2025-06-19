// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/buildpipeline"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/stretchr/testify/assert"
)

const (
	baseImageDistroAzureLinux = "azurelinux"

	// Azure Linux versions
	baseImageVersionAzl2 = "2.0"
	baseImageVersionAzl3 = "3.0"

	// Azure Linux variants
	baseImageVariantCoreEfi   = "core-efi"
	baseImageVariantBareMetal = "bare-metal"
)

type testBaseImageInfo struct {
	Name      string
	Distro    string
	Version   string
	Variant   string
	ParamName string
	Param     *string
}

var (
	testBaseImageAzl2CoreEfi = testBaseImageInfo{
		Name:      "AzureLinux2CoreEfi",
		Distro:    baseImageDistroAzureLinux,
		Version:   baseImageVersionAzl2,
		Variant:   baseImageVariantCoreEfi,
		ParamName: "base-image-core-efi-azl2",
		Param:     baseImageCoreEfiAzl2,
	}

	testBaseImageAzl3CoreEfi = testBaseImageInfo{
		Name:      "AzureLinux3CoreEfi",
		Distro:    baseImageDistroAzureLinux,
		Version:   baseImageVersionAzl3,
		Variant:   baseImageVariantCoreEfi,
		ParamName: "base-image-core-efi-azl3",
		Param:     baseImageCoreEfiAzl3,
	}

	testBaseImageAzl2BareMetal = testBaseImageInfo{
		Name:      "AzureLinux2BareMetal",
		Distro:    baseImageDistroAzureLinux,
		Version:   baseImageVersionAzl2,
		Variant:   baseImageVariantBareMetal,
		ParamName: "base-image-bare-metal-azl2",
		Param:     baseImageBareMetalAzl2,
	}

	testBaseImageAzl3BareMetal = testBaseImageInfo{
		Name:      "AzureLinux3BareMetal",
		Distro:    baseImageDistroAzureLinux,
		Version:   baseImageVersionAzl3,
		Variant:   baseImageVariantBareMetal,
		ParamName: "base-image-bare-metal-azl3",
		Param:     baseImageBareMetalAzl3,
	}

	baseImageAll = []testBaseImageInfo{
		testBaseImageAzl2CoreEfi,
		testBaseImageAzl3CoreEfi,
		testBaseImageAzl2BareMetal,
		testBaseImageAzl3BareMetal,
	}

	defaultBaseImagePriorityList = []testBaseImageInfo{
		testBaseImageAzl2CoreEfi,
		testBaseImageAzl2BareMetal,
		testBaseImageAzl3CoreEfi,
		testBaseImageAzl3BareMetal,
	}
)

var (
	baseImageCoreEfiAzl2   = flag.String("base-image-core-efi-azl2", "", "An Azure Linux 2.0 core-efi image to use as a base image.")
	baseImageCoreEfiAzl3   = flag.String("base-image-core-efi-azl3", "", "An Azure Linux 3.0 core-efi image to use as a base image.")
	baseImageBareMetalAzl2 = flag.String("base-image-bare-metal-azl2", "", "An Azure Linux 2.0 bare-metal image to use as a base image.")
	baseImageBareMetalAzl3 = flag.String("base-image-bare-metal-azl3", "", "An Azure Linux 3.0 bare-metal image to use as a base image.")
)

var (
	testDir    string
	tmpDir     string
	workingDir string
)

func TestMain(m *testing.M) {
	var err error

	logger.InitStderrLog()

	flag.Parse()

	workingDir, err = os.Getwd()
	if err != nil {
		logger.Log.Panicf("Failed to get working directory, error: %s", err)
	}

	testDir = filepath.Join(workingDir, "testdata")
	tmpDir = filepath.Join(workingDir, "_tmp")

	err = os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil {
		logger.Log.Panicf("Failed to create tmp directory, error: %s", err)
	}

	retVal := m.Run()

	err = os.RemoveAll(tmpDir)
	if err != nil {
		logger.Log.Warnf("Failed to cleanup tmp dir (%s). Error: %s", tmpDir, err)
	}

	os.Exit(retVal)
}

func checkSkipForCustomizeImageRequirements(t *testing.T) {
	if !buildpipeline.IsRegularBuild() {
		t.Skip("loopback block device not available")
	}

	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}
}

// Skip the test if requirements for testing CustomizeImage() are not met.
func checkSkipForCustomizeImage(t *testing.T, baseImage testBaseImageInfo) string {
	checkSkipForCustomizeImageRequirements(t)

	if baseImage.Param == nil || *baseImage.Param == "" {
		t.Skipf("--%s is required for this test", baseImage.ParamName)
	}

	return *baseImage.Param
}

func checkSkipForCustomizeDefaultImage(t *testing.T) (string, testBaseImageInfo) {
	checkSkipForCustomizeImageRequirements(t)

	for _, imageInfo := range defaultBaseImagePriorityList {
		if imageInfo.Param != nil && *imageInfo.Param != "" {
			return *imageInfo.Param, imageInfo
		}
	}

	t.Skipf("--%s is required for this test", defaultBaseImagePriorityList[0].ParamName)
	return "", testBaseImageInfo{}
}

func getDownloadedRpmsDir(t *testing.T, azureLinuxVersion string) string {
	downloadedRpmsDir := filepath.Join(testDir, "testrpms/downloadedrpms", azureLinuxVersion)
	dirExists, err := file.DirExists(downloadedRpmsDir)
	if !assert.NoErrorf(t, err, "cannot access downloaded RPMs dir (%s)", downloadedRpmsDir) {
		t.FailNow()
	}
	if !assert.True(t, dirExists) {
		t.Logf("test requires offline RPMs")
		t.Logf("please run toolkit/tools/pkg/imagecustomizerlib/testdata/testrpms/download-test-rpms.sh -t %s",
			azureLinuxVersion)
		t.FailNow()
	}

	return downloadedRpmsDir
}

func getDownloadedRpmsRepoFile(t *testing.T, azureLinuxVersion string, withGpgKey bool) string {
	dir := getDownloadedRpmsDir(t, azureLinuxVersion)

	suffix := "nokey"
	if withGpgKey {
		suffix = "withkey"
	}

	repoFile := filepath.Join(dir, "../", fmt.Sprintf("rpms-%s-%s.repo", azureLinuxVersion, suffix))
	return repoFile
}
