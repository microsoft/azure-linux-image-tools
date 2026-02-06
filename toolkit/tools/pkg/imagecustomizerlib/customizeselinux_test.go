// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageSELinux(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageSELinuxHelper(t, "TestCustomizeImageSELinux"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageSELinuxHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

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
	imageConnection, err := connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, false, []string{"security=selinux", "selinux=1", "enforcing=1"}, []string{})
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
	imageConnection, err = connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, false, []string{}, []string{"security=selinux", "selinux=1", "enforcing=1"})
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
	imageConnection, err = connectToAzureLinuxCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Verify bootloader config.
	verifyKernelCommandLine(t, imageConnection, false, []string{"security=selinux", "selinux=1"}, []string{"enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "permissive")
}

func TestCustomizeImageSELinuxAndPartitions(t *testing.T) {
	for _, baseImageInfo := range baseImageAzureLinuxAll {
		t.Run(baseImageInfo.Name, func(t *testing.T) {
			testCustomizeImageSELinuxAndPartitionsHelper(t, "TestCustomizeImageSELinuxAndPartitions"+baseImageInfo.Name, baseImageInfo)
		})
	}
}

func testCustomizeImageSELinuxAndPartitionsHelper(t *testing.T, testName string, baseImageInfo testBaseImageInfo) {
	baseImage := checkSkipForCustomizeImage(t, baseImageInfo)

	testTmpDir := filepath.Join(tmpDir, testName)
	defer os.RemoveAll(testTmpDir)

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
	verifyKernelCommandLine(t, imageConnection, false, []string{"security=selinux", "selinux=1"}, []string{"enforcing=1"})
	verifySELinuxConfigFile(t, imageConnection, "enforcing")

	// Verify packages are installed.
	ensureFilesExist(t, imageConnection, "/etc/selinux/targeted", "/var/lib/selinux/targeted/active/modules",
		"/usr/bin/seinfo", "/usr/sbin/semanage")
}

func TestCustomizeImageSELinuxNoPolicy(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageSELinuxNoPolicy")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.qcow2")

	configFile := ""
	switch baseImageInfo.Variant {
	case baseImageAzureLinuxVariantCoreEfi:
		configFile = filepath.Join(testDir, "selinux-enforcing-nopackages.yaml")
	case baseImageAzureLinuxVariantBareMetal:
		configFile = filepath.Join(testDir, "selinux-enforcing-removepackages.yaml")
	}

	// Customize image.
	err := CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)

	switch baseImageInfo.Variant {
	case baseImageAzureLinuxVariantCoreEfi:
		assert.ErrorContains(t, err, "SELinux is enabled but policy file is missing (file='/etc/selinux/config')")
		assert.ErrorContains(t, err, "please ensure an SELinux policy is installed")
		assert.ErrorContains(t, err, "the 'selinux-policy' package provides the default policy")

	case baseImageAzureLinuxVariantBareMetal:
		// The /etc/selinux/config file survives the removal of the selinux-policy package.
		// So, the error is different.
		assert.ErrorContains(t, err, "etc/selinux/targeted/contexts/files/file_contexts: No such file or directory")
	}
}

func verifyKernelCommandLine(t *testing.T, imageConnection *imageconnection.ImageConnection, hasUkis bool,
	existsArgs []string, notExistsArgs []string,
) {
	var grubCfgContents string

	if hasUkis {
		// UKI image - extract cmdline from UKI files
		ukiDir := filepath.Join(imageConnection.Chroot().RootDir(), "boot/efi/EFI/Linux")
		cmdlineFromUki, err := extractCmdlineFromUkiForTest(ukiDir)
		if err != nil {
			t.Fatalf("Failed to extract cmdline from UKI: %v", err)
			return
		}
		grubCfgContents = cmdlineFromUki
	} else {
		// GRUB image - read grub.cfg
		grubCfgFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/boot/grub2/grub.cfg")
		contents, err := file.Read(grubCfgFilePath)
		if err != nil {
			t.Fatalf("Failed to read grub.cfg: %v", err)
			return
		}
		grubCfgContents = contents
	}

	for _, existsArg := range existsArgs {
		if hasUkis {
			// UKI cmdline is a plain string of args (no "linux" keyword)
			assert.Containsf(t, grubCfgContents, existsArg,
				"ensure kernel command arg exists (%s)", existsArg)
		} else {
			// GRUB cfg has "linux /boot/vmlinuz... args" format
			assert.Regexpf(t, fmt.Sprintf("linux.* %s ", regexp.QuoteMeta(existsArg)), grubCfgContents,
				"ensure kernel command arg exists (%s)", existsArg)
		}
	}

	for _, notExistsArg := range notExistsArgs {
		assert.NotContainsf(t, grubCfgContents, notExistsArg,
			"ensure kernel command arg not exists (%s)", notExistsArg)
	}
}

func extractCmdlineFromUkiForTest(ukiDir string) (string, error) {
	files, err := os.ReadDir(ukiDir)
	if err != nil {
		return "", fmt.Errorf("failed to read UKI directory: %w", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".efi") {
			ukiPath := filepath.Join(ukiDir, f.Name())

			tempDir, err := os.MkdirTemp("", "test-cmdline-extraction-*")
			if err != nil {
				return "", fmt.Errorf("failed to create temp directory: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Use production code to extract cmdline (handles main UKI and all addons)
			cmdline, err := extractCmdlineFromUkiWithObjcopy(ukiPath, tempDir)
			if err != nil {
				return "", fmt.Errorf("failed to extract cmdline: %w", err)
			}

			return cmdline, nil
		}
	}

	return "", fmt.Errorf("no UKI files found or cmdline not extracted")
}

func verifySELinuxConfigFile(t *testing.T, imageConnection *imageconnection.ImageConnection, mode string) {
	selinuxConfigPath := filepath.Join(imageConnection.Chroot().RootDir(), "/etc/selinux/config")
	selinuxConfigContents, err := file.Read(selinuxConfigPath)
	assert.NoError(t, err, "read SELinux config file")
	assert.Regexp(t, fmt.Sprintf("(?m)^SELINUX=%s$", regexp.QuoteMeta(mode)), selinuxConfigContents)
}
