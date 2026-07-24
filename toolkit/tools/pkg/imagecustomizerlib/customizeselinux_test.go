// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	configFile := filepath.Join(testDir, selinuxForceEnforcingConfigFile(t, baseImageInfo))
	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
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
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
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
	configFile = filepath.Join(testDir, selinuxPermissiveConfigFile(t, baseImageInfo))
	err = basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, outImageFilePath, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
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
	configFile := filepath.Join(testDir, partitionsSELinuxEnforcingConfigFile(t, baseImageInfo))
	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
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
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	if baseImageInfo.Version == baseImageVersionAzl4 {
		// On Azure Linux 4.0, selinux-policy is installed by default and cannot be removed
		// because selinux-policy-targeted (a protected package, also installed by default) depends on it,
		// so the no-policy scenario this test exercises is unreachable.
		t.Skip("Azure Linux 4.0 ships selinux-policy by default and cannot remove it")
	}

	testTmpDir := filepath.Join(tmpDir, "TestCustomizeImageSELinuxNoPolicy")
	defer os.RemoveAll(testTmpDir)

	buildDir := filepath.Join(testTmpDir, "build")
	outImageFilePath := filepath.Join(testTmpDir, "image.qcow2")
	configFile := filepath.Join(testDir, "selinux-enforcing-nopackages.yaml")

	// Customize image.
	err := basicCustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, outImageFilePath, "raw",
		baseImageInfo.PreviewFeatures)
	assert.ErrorContains(t, err, "SELinux is enabled but policy file is missing (file='/etc/selinux/config')")
	assert.ErrorContains(t, err, "please ensure an SELinux policy is installed")
	assert.ErrorContains(t, err, "the 'selinux-policy' package provides the default policy")
}

func verifyKernelCommandLine(t *testing.T, imageConnection *imageconnection.ImageConnection, hasUkis bool,
	existsArgs []string, notExistsArgs []string,
) {
	var kernelToArgs map[string]string

	if hasUkis {
		// UKI image: extract the command line from every UKI (main image + all addons) on the ESP.
		espPath := filepath.Join(imageConnection.Chroot().RootDir(), "boot/efi")

		scratchDir, err := os.MkdirTemp("", "test-cmdline-extraction-*")
		if !assert.NoError(t, err) {
			return
		}
		defer os.RemoveAll(scratchDir)

		kernelToArgs, err = extractKernelCmdlineFromUkiEfis(espPath, scratchDir)
		if !assert.NoError(t, err) {
			return
		}
	} else {
		// GRUB image: read the kernel arguments for every kernel from grub.cfg / BLS entries.
		bootDir := filepath.Join(imageConnection.Chroot().RootDir(), "boot")

		distroHandler, err := NewDistroHandlerFromChroot(imageConnection.Chroot())
		if !assert.NoError(t, err) {
			return
		}

		parsedKernelToArgs, err := distroHandler.ReadGrubConfigLinuxArgs(bootDir)
		if !assert.NoError(t, err) {
			return
		}

		kernelToArgs = grubKernelArgsToStringMap(parsedKernelToArgs)
	}

	assertKernelCmdlineArgs(t, kernelToArgs, existsArgs, notExistsArgs)
}

// partitionsSELinuxEnforcingConfigFile returns the partitions-selinux-enforcing test config file appropriate for the
// given base image version.
func partitionsSELinuxEnforcingConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "partitions-selinux-enforcing-azl3.yaml"
	case baseImageVersionAzl4:
		return "partitions-selinux-enforcing-azl4.yaml"
	default:
		t.Fatalf("unsupported base image version for partitions-selinux-enforcing test: %s", baseImageInfo.Version)
		return ""
	}
}

// selinuxForceEnforcingConfigFile returns the selinux-force-enforcing test config file appropriate for the given base
// image version.
func selinuxForceEnforcingConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "selinux-force-enforcing-azl3.yaml"
	case baseImageVersionAzl4:
		return "selinux-force-enforcing-azl4.yaml"
	default:
		t.Fatalf("unsupported base image version for selinux-force-enforcing test: %s", baseImageInfo.Version)
		return ""
	}
}

// selinuxPermissiveConfigFile returns the selinux-permissive test config file appropriate for the given base
// image version.
func selinuxPermissiveConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "selinux-permissive-azl3.yaml"
	case baseImageVersionAzl4:
		return "selinux-permissive-azl4.yaml"
	default:
		t.Fatalf("unsupported base image version for selinux-permissive test: %s", baseImageInfo.Version)
		return ""
	}
}

func verifySELinuxConfigFile(t *testing.T, imageConnection *imageconnection.ImageConnection, mode string) {
	selinuxConfigPath := filepath.Join(imageConnection.Chroot().RootDir(), "/etc/selinux/config")
	selinuxConfigContents, err := file.Read(selinuxConfigPath)
	assert.NoError(t, err, "read SELinux config file")
	assert.Regexp(t, fmt.Sprintf("(?m)^SELINUX=%s$", regexp.QuoteMeta(mode)), selinuxConfigContents)
}
