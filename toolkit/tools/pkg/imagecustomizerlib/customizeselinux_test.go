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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
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
	verifyKernelCommandLine(t, imageConnection, []string{"security=selinux", "selinux=1"}, []string{"enforcing=1"})
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

	// Check if grub.cfg exists (non-UKI image)
	grubCfgContents, err := file.Read(grubCfgFilePath)
	if err != nil {
		// grub.cfg doesn't exist - this might be a UKI-only image
		// Try to extract cmdline from UKI files
		ukiDir := filepath.Join(imageConnection.Chroot().RootDir(), "boot/efi/EFI/Linux")
		cmdlineFromUki, ukiErr := extractCmdlineFromUkiForTest(ukiDir)
		if ukiErr != nil {
			// Neither grub.cfg nor UKI found - fail
			t.Fatalf("Cannot verify kernel command line: grub.cfg not found and UKI extraction failed: %v", ukiErr)
			return
		}
		grubCfgContents = cmdlineFromUki
	}

	for _, existsArg := range existsArgs {
		assert.Regexpf(t, fmt.Sprintf("(linux.* %s |%s)", regexp.QuoteMeta(existsArg), regexp.QuoteMeta(existsArg)), grubCfgContents,
			"ensure kernel command arg exists (%s)", existsArg)
	}

	for _, notExistsArg := range notExistsArgs {
		assert.NotContainsf(t, grubCfgContents, notExistsArg,
			"ensure kernel command arg not exists (%s)", notExistsArg)
	}
}

// extractCmdlineFromUkiForTest extracts command line from first UKI file found
func extractCmdlineFromUkiForTest(ukiDir string) (string, error) {
	files, err := os.ReadDir(ukiDir)
	if err != nil {
		return "", fmt.Errorf("failed to read UKI directory: %w", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".efi") {
			ukiPath := filepath.Join(ukiDir, f.Name())

			// Create a temp directory for extraction
			tempDir, err := os.MkdirTemp("", "test-cmdline-extraction-*")
			if err != nil {
				return "", fmt.Errorf("failed to create temp directory: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Create temp file for cmdline output
			cmdlinePath := filepath.Join(tempDir, "cmdline.txt")

			// Create a temp copy of the UKI file to avoid modifying the original
			tempCopy := filepath.Join(tempDir, "uki-copy.efi")
			input, err := os.ReadFile(ukiPath)
			if err != nil {
				return "", fmt.Errorf("failed to read UKI file: %w", err)
			}
			if err := os.WriteFile(tempCopy, input, 0o644); err != nil {
				return "", fmt.Errorf("failed to write temp UKI file: %w", err)
			}

			// Extract .cmdline section using objcopy
			_, _, err = shell.Execute("objcopy", "--dump-section", ".cmdline="+cmdlinePath, tempCopy)
			if err != nil {
				return "", fmt.Errorf("objcopy failed to extract cmdline: %w", err)
			}

			// Read the extracted cmdline content
			content, err := os.ReadFile(cmdlinePath)
			if err != nil {
				return "", fmt.Errorf("failed to read cmdline file: %w", err)
			}

			return string(content), nil
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
