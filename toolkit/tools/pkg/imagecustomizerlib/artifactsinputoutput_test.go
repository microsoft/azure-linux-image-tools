package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/stretchr/testify/assert"
)

func TestOutputAndInjectArtifacts(t *testing.T) {
	baseImage := checkSkipForCustomizeImage(t, baseImageTypeCoreEfi, baseImageVersionAzl3)

	testTempDir := filepath.Join(tmpDir, "TestOutputAndInjectArtifacts")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	configFile := filepath.Join(testDir, "artifacts-output.yaml")
	outputArtifactsDir := filepath.Join(testDir, "output")

	// Customize image
	err := CustomizeImageWithConfigFile(buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		"" /*outputPXEArtifactsDir*/, true /*useBaseImageRpmRepos*/)
	if !assert.NoError(t, err) {
		return
	}

	// Confirm inject-files.yaml was generated
	injectConfigPath := filepath.Join(outputArtifactsDir, "inject-files.yaml")
	exists, err := file.PathExists(injectConfigPath)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected inject-files.yaml to be generated")

	// Confirm artifacts were outputted
	// Detect boot binary
	bootBinaries, err := filepath.Glob(filepath.Join(outputArtifactsDir, "boot*.efi"))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(bootBinaries), "Expected exactly one boot<arch>.efi binary")
	bootBinary := bootBinaries[0]

	// Detect systemd-boot binary
	systemdBootBinaries, err := filepath.Glob(filepath.Join(outputArtifactsDir, "systemd-boot*.efi"))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(systemdBootBinaries), "Expected exactly one systemd-boot<arch>.efi binary")
	systemdBootBinary := systemdBootBinaries[0]

	// Detect unsigned UKIs
	ukiUnsignedFiles, err := filepath.Glob(filepath.Join(outputArtifactsDir, "vmlinuz-*.unsigned.efi"))
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(ukiUnsignedFiles), 1, "Expected at least one unsigned UKI")

	// Simulate signed boot & systemd-boot
	for _, src := range []string{bootBinary, systemdBootBinary} {
		dst := replaceSuffix(src, ".efi", ".signed.efi")
		err := file.Copy(src, dst)
		assert.NoError(t, err, "Failed to simulate signed file: %s", filepath.Base(src))
	}

	// Simulate signed UKIs
	for _, src := range ukiUnsignedFiles {
		dst := replaceSuffix(src, ".unsigned.efi", ".signed.efi")
		err := file.Copy(src, dst)
		assert.NoError(t, err, "Failed to simulate signed UKI: %s", filepath.Base(src))
	}

	// Inject artifacts into a fresh copy of the raw image
	err = InjectFilesWithConfigFile(buildDir, injectConfigPath, outImageFilePath, "", "")
	if !assert.NoError(t, err) {
		return
	}

	// Mount injected image and verify one file was injected
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
	}

	// Connect to customized image.
	imageConnection, err := connectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check the injected files
	// shim
	expectedInjectedShim := filepath.Join(imageConnection.chroot.RootDir(), "EFI", "BOOT", filepath.Base(bootBinary))
	exists, err = file.PathExists(expectedInjectedShim)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected injected shim to exist: %s", expectedInjectedShim)

	// systemd-boot
	expectedInjectedSystemdBoot := filepath.Join(imageConnection.chroot.RootDir(), "EFI", "systemd", filepath.Base(systemdBootBinary))
	exists, err = file.PathExists(expectedInjectedSystemdBoot)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected injected systemd-boot to exist: %s", expectedInjectedSystemdBoot)

	// UKI(s)
	for _, src := range ukiUnsignedFiles {
		signedName := filepath.Base(replaceSuffix(src, ".unsigned.efi", ".efi"))
		expectedInjectedUKI := filepath.Join(imageConnection.chroot.RootDir(), "EFI", "Linux", signedName)

		exists, err = file.PathExists(expectedInjectedUKI)
		assert.NoError(t, err)
		assert.True(t, exists, "Expected injected UKI to exist: %s", expectedInjectedUKI)
	}

	// Cleanup output directory after test
	err = os.RemoveAll(outputArtifactsDir)
	assert.NoError(t, err, "Failed to clean up output artifacts directory: %s", outputArtifactsDir)
}
