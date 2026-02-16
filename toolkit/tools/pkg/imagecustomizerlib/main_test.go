// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/sirupsen/logrus"
)

const (
	baseImageDistroAzureLinux = "azurelinux"
	baseImageDistroUbuntu     = "ubuntu"

	// Azure Linux versions
	baseImageVersionAzl2 = "2.0"
	baseImageVersionAzl3 = "3.0"

	// Ubuntu versions
	baseImageVersionUbuntu2204 = "22.04"
	baseImageVersionUbuntu2404 = "24.04"

	// Azure Linux variants
	baseImageAzureLinuxVariantCoreEfi   = "core-efi"
	baseImageAzureLinuxVariantBareMetal = "bare-metal"

	// Ubuntu variants
	baseImageVariantUbuntuAzureCloud = "azure-cloud"

	// Default shells
	azureLinuxDefaultShell = "/bin/bash"
	ubuntuDefaultShell     = "/bin/sh"

	// Flag names
	paramBaseImageAzl2CoreEfi          = "base-image-core-efi-azl2"
	paramBaseImageAzl3CoreEfi          = "base-image-core-efi-azl3"
	paramBaseImageAzl2BareMetal        = "base-image-bare-metal-azl2"
	paramBaseImageAzl3BareMetal        = "base-image-bare-metal-azl3"
	paramBaseImageUbuntu2204AzureCloud = "base-image-azure-cloud-ubuntu2204"
	paramBaseImageUbuntu2404AzureCloud = "base-image-azure-cloud-ubuntu2404"
	paramLogLevel                      = "log-level"
)

type testBaseImageInfo struct {
	Name            string
	Distro          string
	Version         string
	Variant         string
	ParamName       string
	Param           *string
	MountPoints     []testutils.MountPoint
	DefaultShell    string
	PreviewFeatures []imagecustomizerapi.PreviewFeature
}

var (
	azureLinuxCoreEfiMountPoints = []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	azureLinuxCoreLegacyMountPoints = []testutils.MountPoint{
		{
			PartitionNum:   2,
			Path:           "/",
			FileSystemType: "ext4",
		},
	}

	ubuntuAzureCloudMountPoints = []testutils.MountPoint{
		{
			PartitionNum:   1,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   15,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	testBaseImageAzl2CoreEfi = testBaseImageInfo{
		Name:         "AzureLinux2CoreEfi",
		Distro:       baseImageDistroAzureLinux,
		Version:      baseImageVersionAzl2,
		Variant:      baseImageAzureLinuxVariantCoreEfi,
		ParamName:    paramBaseImageAzl2CoreEfi,
		Param:        baseImageCoreEfiAzl2,
		MountPoints:  azureLinuxCoreEfiMountPoints,
		DefaultShell: azureLinuxDefaultShell,
	}

	testBaseImageAzl3CoreEfi = testBaseImageInfo{
		Name:         "AzureLinux3CoreEfi",
		Distro:       baseImageDistroAzureLinux,
		Version:      baseImageVersionAzl3,
		Variant:      baseImageAzureLinuxVariantCoreEfi,
		ParamName:    paramBaseImageAzl3CoreEfi,
		Param:        baseImageCoreEfiAzl3,
		MountPoints:  azureLinuxCoreEfiMountPoints,
		DefaultShell: azureLinuxDefaultShell,
	}

	testBaseImageAzl2BareMetal = testBaseImageInfo{
		Name:         "AzureLinux2BareMetal",
		Distro:       baseImageDistroAzureLinux,
		Version:      baseImageVersionAzl2,
		Variant:      baseImageAzureLinuxVariantBareMetal,
		ParamName:    paramBaseImageAzl2BareMetal,
		Param:        baseImageBareMetalAzl2,
		MountPoints:  azureLinuxCoreLegacyMountPoints,
		DefaultShell: azureLinuxDefaultShell,
	}

	testBaseImageAzl3BareMetal = testBaseImageInfo{
		Name:         "AzureLinux3BareMetal",
		Distro:       baseImageDistroAzureLinux,
		Version:      baseImageVersionAzl3,
		Variant:      baseImageAzureLinuxVariantBareMetal,
		ParamName:    paramBaseImageAzl3BareMetal,
		Param:        baseImageBareMetalAzl3,
		MountPoints:  azureLinuxCoreLegacyMountPoints,
		DefaultShell: azureLinuxDefaultShell,
	}

	testBaseImageUbuntu2204AzureCloud = testBaseImageInfo{
		Name:         "Ubuntu2204AzureCloud",
		Distro:       baseImageDistroUbuntu,
		Version:      baseImageVersionUbuntu2204,
		Variant:      baseImageVariantUbuntuAzureCloud,
		ParamName:    paramBaseImageUbuntu2204AzureCloud,
		Param:        baseImageUbuntuAzureCloud2204,
		MountPoints:  ubuntuAzureCloudMountPoints,
		DefaultShell: ubuntuDefaultShell,
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureUbuntu2204,
		},
	}

	testBaseImageUbuntu2404AzureCloud = testBaseImageInfo{
		Name:         "Ubuntu2404AzureCloud",
		Distro:       baseImageDistroUbuntu,
		Version:      baseImageVersionUbuntu2404,
		Variant:      baseImageVariantUbuntuAzureCloud,
		ParamName:    paramBaseImageUbuntu2404AzureCloud,
		Param:        baseImageUbuntuAzureCloud2404,
		MountPoints:  ubuntuAzureCloudMountPoints,
		DefaultShell: ubuntuDefaultShell,
		PreviewFeatures: []imagecustomizerapi.PreviewFeature{
			imagecustomizerapi.PreviewFeatureUbuntu2404,
		},
	}

	baseImageAzureLinuxAll = []testBaseImageInfo{
		testBaseImageAzl2CoreEfi,
		testBaseImageAzl3CoreEfi,
		testBaseImageAzl2BareMetal,
		testBaseImageAzl3BareMetal,
	}

	baseImageUbuntuAll = []testBaseImageInfo{
		testBaseImageUbuntu2404AzureCloud,
		testBaseImageUbuntu2204AzureCloud,
	}

	defaultAzureLinuxPriorityList = []testBaseImageInfo{
		testBaseImageAzl3CoreEfi,
		testBaseImageAzl3BareMetal,
		testBaseImageAzl2CoreEfi,
		testBaseImageAzl2BareMetal,
	}
)

var (
	baseImageCoreEfiAzl2          = flag.String(paramBaseImageAzl2CoreEfi, "", "An Azure Linux 2.0 core-efi image to use as a base image.")
	baseImageCoreEfiAzl3          = flag.String(paramBaseImageAzl3CoreEfi, "", "An Azure Linux 3.0 core-efi image to use as a base image.")
	baseImageBareMetalAzl2        = flag.String(paramBaseImageAzl2BareMetal, "", "An Azure Linux 2.0 bare-metal image to use as a base image.")
	baseImageBareMetalAzl3        = flag.String(paramBaseImageAzl3BareMetal, "", "An Azure Linux 3.0 bare-metal image to use as a base image.")
	baseImageUbuntuAzureCloud2204 = flag.String(paramBaseImageUbuntu2204AzureCloud, "", "An Ubuntu 22.04 Azure cloud image to use as a base image.")
	baseImageUbuntuAzureCloud2404 = flag.String(paramBaseImageUbuntu2404AzureCloud, "", "An Ubuntu 24.04 Azure cloud image to use as a base image.")
	logLevel                      = flag.String(paramLogLevel, "info", "The log level (error, warning, info, debug, or trace)")
)

var (
	testDir             string
	tmpDir              string
	workingDir          string
	testutilsDir        string
	sharedImageCacheDir string

	logMessagesHook *logger.MemoryLogHook
)

func TestMain(m *testing.M) {
	var err error

	logger.InitStderrLog()

	flag.Parse()

	if logLevel != nil {
		err := logger.SetStderrLogLevel(*logLevel)
		if err != nil {
			logger.Log.Panicf("Failed to set log level, error: %s", err)
		}
	}

	logMessagesHook = logger.NewMemoryLogHook()
	logger.Log.Hooks.Add(logMessagesHook)
	logger.Log.SetLevel(logrus.DebugLevel)

	workingDir, err = os.Getwd()
	if err != nil {
		logger.Log.Panicf("Failed to get working directory, error: %s", err)
	}

	testDir = filepath.Join(workingDir, "testdata")
	tmpDir = filepath.Join(workingDir, "_tmp")
	sharedImageCacheDir = filepath.Join(tmpDir, "image-cache")
	testutilsDir = filepath.Join(workingDir, "../../internal/testutils")

	err = os.MkdirAll(sharedImageCacheDir, os.ModePerm)
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

// Skip the test if requirements for testing CustomizeImage() are not met.
func checkSkipForCustomizeImage(t *testing.T, baseImage testBaseImageInfo) string {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	if baseImage.Param == nil || *baseImage.Param == "" {
		t.Skipf("--%s is required for this test", baseImage.ParamName)
	}

	return *baseImage.Param
}

func findFirstAvailableImage(priorityList []testBaseImageInfo) (testBaseImageInfo, bool) {
	for _, imageInfo := range priorityList {
		if imageInfo.Param != nil && *imageInfo.Param != "" {
			return imageInfo, true
		}
	}

	return testBaseImageInfo{}, false
}

func checkSkipForCustomizeDefaultAzureLinuxImage(t *testing.T) (string, testBaseImageInfo) {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	imageInfo, found := findFirstAvailableImage(defaultAzureLinuxPriorityList)
	if !found {
		t.Skipf("--%s is required for this test", defaultAzureLinuxPriorityList[0].ParamName)
	}

	return *imageInfo.Param, imageInfo
}

func checkSkipForCustomizeDefaultImages(t *testing.T) []testBaseImageInfo {
	testutils.CheckSkipForCustomizeImageRequirements(t)

	var images []testBaseImageInfo

	if imageInfo, found := findFirstAvailableImage(defaultAzureLinuxPriorityList); found {
		images = append(images, imageInfo)
	}

	for _, imageInfo := range baseImageUbuntuAll {
		if imageInfo.Param != nil && *imageInfo.Param != "" {
			images = append(images, imageInfo)
		}
	}

	if len(images) == 0 {
		t.Skipf("--%s is required for this test", defaultAzureLinuxPriorityList[0].ParamName)
	}

	return images
}
