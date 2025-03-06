// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestCustomizeImageOverlays(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageOverlays")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "overlays-config.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	// Connect to customized image.
	mountPoints := []mountPoint{
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
		{
			PartitionNum:   4,
			Path:           "/var",
			FileSystemType: "ext4",
		},
	}

	// Connect to customized image.
	imageConnection, err := connectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Read fstab file.
	fstabPath := filepath.Join(imageConnection.chroot.RootDir(), "etc/fstab")
	fstabContents, err := file.Read(fstabPath)
	if !assert.NoError(t, err) {
		return
	}

	// Check for specific overlay configurations in fstab
	assert.Contains(t, fstabContents,
		"overlay /etc overlay lowerdir=/sysroot/etc,"+
			"upperdir=/sysroot/var/overlays/etc/upper,workdir=/sysroot/var/overlays/etc/work,"+
			"x-systemd.requires=/sysroot/var,x-initrd.mount,x-systemd.wanted-by=initrd-fs.target 0 0")

	assert.Contains(t, fstabContents,
		"overlay /media overlay lowerdir=/media:/home,"+
			"upperdir=/overlays/media/upper,workdir=/overlays/media/work 0 0")
}

func TestCustomizeImageOverlaysSELinux(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionDefault)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageOverlaysSELinux")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "overlays-selinux.yaml")

	// Customize image.
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	imageConnection, err := connectToCoreEfiImage(buildDir, outImageFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Read fstab file.
	fstabPath := filepath.Join(imageConnection.chroot.RootDir(), "etc/fstab")
	fstabContents, err := file.Read(fstabPath)
	assert.NoError(t, err)

	// Check for specific overlay configurations in fstab
	assert.Contains(t, fstabContents,
		"overlay /var overlay lowerdir=/var,upperdir=/mnt/overlays/var/upper,workdir=/mnt/overlays/var/work 0 0")

	upperLabel, err := getSELinuxLabel(filepath.Join(imageConnection.chroot.RootDir(), "/mnt/overlays/var/upper"))
	assert.NoError(t, err)

	workLabel, err := getSELinuxLabel(filepath.Join(imageConnection.chroot.RootDir(), "/mnt/overlays/var/work"))
	assert.NoError(t, err)

	assert.Contains(t, upperLabel, ":object_r:var_t:s0")
	assert.Contains(t, workLabel, ":object_r:no_access_t:s0")
}

func getSELinuxLabel(path string) (string, error) {
	stdout, _, err := shell.Execute("ls", "-Zd", path)
	if err != nil {
		return "", fmt.Errorf("failed to get SELinux label (%s):\n%w", path, err)
	}

	// Example stdout:
	//   system_u:object_r:root_t:s0 /
	fields := strings.Fields(stdout)
	seLinuxLabel := fields[0]

	return seLinuxLabel, nil
}
