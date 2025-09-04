package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

const (
	pseudoSignedMarker = "##TEST_MARKER_INJECTED##"
)

func TestOutputAndInjectArtifacts(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
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

	espFiles := verifyAndSignOutputtedArtifacts(t, outputArtifactsDir)

	// Inject artifacts into a fresh copy of the raw image
	injectConfigPath := filepath.Join(outputArtifactsDir, "inject-files.yaml")
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

	verifyInjectedFiles(t, filepath.Join(imageConnection.Chroot().RootDir(), "boot/efi"), espFiles)
}

func TestOutputAndInjectArtifactsCosi(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	if runtime.GOARCH == "arm64" {
		t.Skip("systemd-boot not available on AZL3 ARM64 yet")
	}

	testTempDir := filepath.Join(tmpDir, "TestOutputAndInjectArtifacts")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	cosiFilePath := filepath.Join(testTempDir, "image.cosi")
	originalConfigFile := filepath.Join(testDir, "artifacts-output.yaml")
	configFile := filepath.Join(testTempDir, "artifacts-output.yaml")
	outputArtifactsDir := filepath.Join(testTempDir, "output")
	injectConfigPath := filepath.Join(outputArtifactsDir, "inject-files.yaml")

	// Copy test config to the temp dir so it's isolated
	err = file.Copy(originalConfigFile, configFile)
	assert.NoError(t, err)

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	espFiles := verifyAndSignOutputtedArtifacts(t, outputArtifactsDir)

	// Inject artifacts into image.
	err = InjectFilesWithConfigFile(t.Context(), buildDir, injectConfigPath, outImageFilePath, cosiFilePath, "cosi")
	if !assert.NoError(t, err) {
		return
	}

	// Connect to image.
	partitionsPaths, err := extractPartitionsFromCosi(cosiFilePath, testTempDir)
	if !assert.NoError(t, err) || !assert.Len(t, partitionsPaths, 3) {
		return
	}

	espPartitionPath := filepath.Join(testTempDir, "image_1.raw")
	//bootPartitionPath := filepath.Join(testTempDir, "image_2.raw")
	//rootPartitionPath := filepath.Join(testTempDir, "image_3.raw")

	espDevice, err := safeloopback.NewLoopback(espPartitionPath)
	if !assert.NoError(t, err) {
		return
	}
	defer espDevice.Close()

	espMountPath := filepath.Join(testTempDir, "esppartition")
	espMount, err := safemount.NewMount(espDevice.DevicePath(), espMountPath, "vfat", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer espMount.Close()

	verifyInjectedFiles(t, espMountPath, espFiles)
}

func verifyAndSignOutputtedArtifacts(t *testing.T, outputArtifactsDir string) []string {
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
	espFiles := []string(nil)

	for _, entry := range injectConfig.InjectFiles {
		switch {
		case strings.HasPrefix(entry.Destination, "/EFI/BOOT/boot") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./boot") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasShim = true

			espFiles = append(espFiles, entry.Destination)

		case strings.HasPrefix(entry.Destination, "/EFI/systemd/systemd-boot") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./systemd-boot") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasSystemdBoot = true

			espFiles = append(espFiles, entry.Destination)

		case strings.HasPrefix(entry.Destination, "/EFI/Linux/vmlinuz") &&
			strings.HasSuffix(entry.Destination, ".efi") &&
			strings.HasPrefix(entry.Source, "./vmlinuz") &&
			strings.HasSuffix(entry.Source, ".signed.efi"):
			hasUKI = true

			espFiles = append(espFiles, entry.Destination)
		}

		// Check that the unsigned file exists.
		unsignedPath := filepath.Join(outputArtifactsDir, entry.UnsignedSource)
		_, err := os.Stat(unsignedPath)
		assert.NoErrorf(t, err, "failed to check if unsigned file exists ('%s')", unsignedPath)

		// Pseudo sign the file.
		signedPath := filepath.Join(outputArtifactsDir, entry.Source)
		err = pseudoSignFile(unsignedPath, signedPath)
		assert.NoErrorf(t, err, "pseduo sign file failed (unsignedPath='%s', signedPath='%s')", unsignedPath, signedPath)
	}

	// Ensure all the expected files were seen.
	assert.Equal(t, 3, len(injectConfig.InjectFiles))
	assert.True(t, hasShim, "Expected an inject entry for shim")
	assert.True(t, hasSystemdBoot, "Expected an inject entry for systemd-boot")
	assert.True(t, hasUKI, "Expected at least one inject entry for UKI")

	return espFiles
}

func verifyInjectedFiles(t *testing.T, partitionDir string, partitionFiles []string) {
	// Check the injected files.
	for _, filePath := range partitionFiles {
		injectedFilePath := filepath.Join(partitionDir, filePath)

		contains, err := fileContainsMarker(injectedFilePath, pseudoSignedMarker)
		assert.NoError(t, err)
		assert.Truef(t, contains, "Expected injected file to exist (%s)", injectedFilePath)
	}
}

func pseudoSignFile(inputPath string, outputPath string) error {
	err := file.Copy(inputPath, outputPath)
	if err != nil {
		return err
	}

	err = appendMarker(outputPath, pseudoSignedMarker)
	if err != nil {
		return err
	}

	return nil
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
