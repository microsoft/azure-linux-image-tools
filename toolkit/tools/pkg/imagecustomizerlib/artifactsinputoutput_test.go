package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestOutputAndInjectArtifacts(t *testing.T) {
	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, "TestOutputAndInjectArtifacts")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	originalConfigFile := filepath.Join(testDir, "artifacts-output.yaml")
	configFile := filepath.Join(testTempDir, "artifacts-output.yaml")
	outputArtifactsDir := filepath.Join(testTempDir, "output")

	// Copy test config to the temp dir so it's isolated
	err = file.Copy(originalConfigFile, configFile)
	assert.NoError(t, err)

	// Customize image
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Confirm inject-files.yaml was generated
	injectConfigPath := filepath.Join(outputArtifactsDir, "inject-files.yaml")
	exists, err := file.PathExists(injectConfigPath)
	assert.NoError(t, err)
	assert.True(t, exists, "Expected inject-files.yaml to be generated")

	var injectConfig imagecustomizerapi.InjectFilesConfig
	err = imagecustomizerapi.UnmarshalYamlFile(injectConfigPath, &injectConfig)
	assert.NoError(t, err)

	// Check previewFeatures
	assert.Contains(t, injectConfig.PreviewFeatures, imagecustomizerapi.PreviewFeatureInjectFiles, "Expected previewFeatures to include 'inject-files'")

	// Check artifacts
	hasShim := false
	hasSystemdBoot := false
	hasUKI := false

	for _, entry := range injectConfig.InjectFiles {
		switch {
		case strings.HasPrefix(entry.Destination, "/EFI/BOOT/boot") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./boot") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasShim = true
		case strings.HasPrefix(entry.Destination, "/EFI/systemd/systemd-boot") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./systemd-boot") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasSystemdBoot = true
		case strings.HasPrefix(entry.Destination, "/EFI/Linux/vmlinuz") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./vmlinuz") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasUKI = true
		}
	}

	assert.True(t, hasShim, "Expected an inject entry for shim")
	assert.True(t, hasSystemdBoot, "Expected an inject entry for systemd-boot")
	assert.True(t, hasUKI, "Expected at least one inject entry for UKI")

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
	ukiFiles, err := filepath.Glob(filepath.Join(outputArtifactsDir, "vmlinuz-*.efi"))
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(ukiFiles), 1, "Expected at least one UKI file")

	// Simulate signed boot & systemd-boot
	marker := "##TEST_MARKER_INJECTED##"
	for _, src := range []string{bootBinary, systemdBootBinary} {
		dst := replaceSuffix(src, ".efi", ".signed.efi")
		err := file.Copy(src, dst)
		assert.NoError(t, err)

		err = appendMarker(dst, marker)
		assert.NoError(t, err)
	}

	// Simulate signed UKIs
	for _, src := range ukiFiles {
		dst := replaceSuffix(src, ".efi", ".signed.efi")
		err := file.Copy(src, dst)
		assert.NoError(t, err)
	}

	// Inject artifacts into a fresh copy of the raw image
	err = InjectFilesWithConfigFile(t.Context(), buildDir, injectConfigPath, outImageFilePath, "", "")
	if !assert.NoError(t, err) {
		return
	}

	// Mount injected image and verify one file was injected
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

	// Connect to customized image.
	imageConnection, err := testutils.ConnectToImage(buildDir, outImageFilePath, false /*includeDefaultMounts*/, mountPoints)
	if !assert.NoError(t, err) {
		return
	}
	defer imageConnection.Close()

	// Check the injected files
	// shim
	expectedInjectedShim := filepath.Join(imageConnection.Chroot().RootDir(), "EFI", "BOOT", filepath.Base(bootBinary))
	contains, err := fileContainsMarker(expectedInjectedShim, marker)
	assert.NoError(t, err)
	assert.True(t, contains, "Expected injected shim to exist:\n%s", expectedInjectedShim)

	// systemd-boot
	expectedInjectedSystemdBoot := filepath.Join(imageConnection.Chroot().RootDir(), "EFI", "systemd", filepath.Base(systemdBootBinary))
	contains, err = fileContainsMarker(expectedInjectedSystemdBoot, marker)
	assert.NoError(t, err)
	assert.True(t, contains, "Expected injected systemd-boot to exist:\n%s", expectedInjectedSystemdBoot)

	// UKI(s)
	for _, src := range ukiFiles {
		expectedInjectedUKI := filepath.Join(imageConnection.Chroot().RootDir(), "EFI", "Linux", filepath.Base(src))

		exists, err = file.PathExists(expectedInjectedUKI)
		assert.NoError(t, err)
		assert.True(t, exists, "Expected injected UKI to exist:\n%s", expectedInjectedUKI)
	}
}

func appendMarker(path string, marker string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(marker)
	return err
}

func fileContainsMarker(path string, marker string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(content), marker), nil
}
