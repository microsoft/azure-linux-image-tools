// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageSELinux(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageSELinuxHelper(t, "TestCustomizeImageSELinux"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageSELinuxHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image: SELinux enforcing.
	// This tests enabling SELinux on a non-SELinux image.
	configFile := filepath.Join(testDir, "selinux-force-enforcing.yaml")
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, []string{"security=selinux", "selinux=1", "enforcing=1"}, []string{})
	verifySELinuxConfigFile(t, imageConnection, "enforcing")

	// Verify packages are installed.
	ensureFilesExist(t, imageConnection, "/etc/selinux/targeted", "/var/lib/selinux/targeted/active/modules",
		"/usr/bin/seinfo", "/usr/sbin/semanage")

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	// Customize image: SELinux disabled.
	// This tests disabling (but not removing) SELinux on an SELinux enabled image.
	configFile = filepath.Join(testDir, "selinux-disabled.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	imageConnection, err = connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, []string{}, []string{"security=selinux", "selinux=1", "enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "disabled")

	// Verify packages are still installed.
	ensureFilesExist(t, imageConnection, "/etc/selinux/targeted", "/var/lib/selinux/targeted/active/modules",
		"/usr/bin/seinfo", "/usr/sbin/semanage")

	err = imageConnection.CleanClose()
	if !assert.NoError(t, err) {
		return
	}

	// Customize image: SELinux permissive.
	// This tests enabling SELinux on an image with SELinux installed but disabled.
	configFile = filepath.Join(testDir, "selinux-permissive.yaml")
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	imageConnection, err = connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, []string{"security=selinux", "selinux=1"}, []string{"enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "permissive")
}

func TestCustomizeImageSELinuxAndPartitions(t *testing.T) {
	for _, baseImageInfo := range baseImageAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageSELinuxAndPartitionsHelper(t, "TestCustomizeImageSELinuxAndPartitions"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageSELinuxAndPartitionsHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.raw")

	// Customize image: SELinux enforcing.
	// This tests enabling SELinux on a non-SELinux image.
	configFile := filepath.Join(testDir, "partitions-selinux-enforcing.yaml")
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	mountPoints := []testutils.MountPoint{
		{
			PartitionNum:   3,
			Path:           "/",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   2,
			Path:           "/boot",
			FileSystemType: "ext4",
		},
		{
			PartitionNum:   1,
			Path:           "/boot/efi",
			FileSystemType: "vfat",
		},
	}

	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, []string{"security=selinux", "selinux=1"}, []string{"enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "enforcing")

	// Verify packages are installed.
	ensureFilesExist(t, imageConnection, "/etc/selinux/targeted", "/var/lib/selinux/targeted/active/modules",
		"/usr/bin/seinfo", "/usr/sbin/semanage")
}

func TestCustomizeImageSELinuxNoPolicy(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageSELinuxNoPolicy")
	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.qcow2")

	configFile := ""
	switch baseImageInfo.Variant {
	case baseImageVariantCoreEfi:
		configFile = filepath.Join(testDir, "selinux-enforcing-nopackages.yaml")
	case baseImageVariantBareMetal:
		configFile = filepath.Join(testDir, "selinux-enforcing-removepackages.yaml")
	}

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)

	switch baseImageInfo.Variant {
	case baseImageVariantCoreEfi:
		assert.ErrorContains(t, err, "SELinux is enabled but policy file is missing (file='/etc/selinux/config')")
		assert.ErrorContains(t, err, "please ensure an SELinux policy is installed")
		assert.ErrorContains(t, err, "the 'selinux-policy' package provides the default policy")

	case baseImageVariantBareMetal:
		// The /etc/selinux/config file survives the removal of the selinux-policy package.
		// So, the error is different.
		assert.ErrorContains(t, err, "etc/selinux/targeted/contexts/files/file_contexts: No such file or directory")
	}
}

func verifyKernelCommandLine(t *testing.T, imageConnection *imageconnection.ImageConnection, existsArgs []string,
	notExistsArgs []string,
) {
	grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
	grubCfgContents, err := file.Read(grubCfgFilePath)
	assert.NoError(t, err, "read grub.cfg file")

	for _, existsArg := range existsArgs {
		assert.Regexpf(t, fmt.Sprintf("linux.* %s ", regexp.QuoteMeta(existsArg)), grubCfgContents,
			"ensure kernel command arg exists (%s)", existsArg)
	}

	for _, notExistsArg := range notExistsArgs {
		assert.NotRegexpf(t, fmt.Sprintf("linux.* %s ", regexp.QuoteMeta(notExistsArg)), grubCfgContents,
			"ensure kernel command arg not exists (%s)", notExistsArg)
	}
}

func verifySELinuxConfigFile(t *testing.T, imageConnection *imageconnection.ImageConnection, mode string) {
	selinuxConfigPath := filepath.Join(imageConnection.Chroot().RootDir(), "/etc/selinux/config")
	selinuxConfigContents, err := file.Read(selinuxConfigPath)
	assert.NoError(t, err, "read SELinux config file")
	assert.Regexp(t, fmt.Sprintf("(?m)^SELINUX=%s$", regexp.QuoteMeta(mode)), selinuxConfigContents)
}
