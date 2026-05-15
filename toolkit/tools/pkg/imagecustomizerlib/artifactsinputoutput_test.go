package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/testutils"
	"github.com/stretchr/testify/assert"
)

const (
	pseudoSignedMarker = "##TEST_MARKER_INJECTED##"
)

func TestOutputAndInjectArtifacts(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}
	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("Azure Linux 4.0 does not yet support this test")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, "TestOutputAndInjectArtifacts")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	buildDirCustomize := filepath.Join(buildDir, "customize")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	originalConfigFile := filepath.Join(testDir, artifactsOutputConfigFile(t, baseImageInfo))
	configFile := filepath.Join(testTempDir, "artifacts-output.yaml")
	outputArtifactsDir := filepath.Join(testTempDir, "output")

	// Copy test config to the temp dir so it's isolated
	err = file.Copy(originalConfigFile, configFile)
	assert.NoError(t, err)

	// Customize image
	err = CustomizeImageWithConfigFile(t.Context(), buildDirCustomize, configFile, baseImage, nil, outImageFilePath, "raw",
		true /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	espFiles := verifyAndSignOutputtedArtifacts(t, outputArtifactsDir, false)

	// Use new buildDir to ensure the buildDir is created if it doesn't exist.
	buildDirInject := filepath.Join(buildDir, "inject")

	// Inject artifacts into a fresh copy of the raw image
	injectConfigPath := filepath.Join(outputArtifactsDir, "inject-files.yaml")
	options := InjectFilesOptions{
		BuildDir:       buildDirInject,
		InputImageFile: outImageFilePath,
	}
	err = InjectFilesWithConfigFile(t.Context(), injectConfigPath, options)
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

// artifactsOutputConfigFile returns the artifacts-output test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func artifactsOutputConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "artifacts-output-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("artifacts-output-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for artifacts-output test: %s", baseImageInfo.Version)
		return ""
	}
}

// artifactsOutputVerityConfigFile returns the artifacts-output-verity test config file appropriate for the
// given base image version (azl3 vs azl4) and host architecture.
func artifactsOutputVerityConfigFile(t *testing.T, baseImageInfo testBaseImageInfo) string {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return "artifacts-output-verity-azl3.yaml"
	case baseImageVersionAzl4:
		return fmt.Sprintf("artifacts-output-verity-%s-azl4.yaml", runtime.GOARCH)
	default:
		t.Fatalf("unsupported base image version for artifacts-output-verity test: %s", baseImageInfo.Version)
		return ""
	}
}

func TestOutputAndInjectArtifactsCosi(t *testing.T) {
	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)
	if baseImageInfo.Version == baseImageVersionAzl2 {
		t.Skip("'systemd-boot' is not available on Azure Linux 2.0")
	}
	if baseImageInfo.Version == baseImageVersionAzl4 {
		t.Skip("Azure Linux 4.0 does not yet support this test")
	}

	ukifyExists, err := file.CommandExists("ukify")
	assert.NoError(t, err)
	if !ukifyExists {
		t.Skip("The 'ukify' command is not available")
	}

	testTempDir := filepath.Join(tmpDir, "TestOutputAndInjectArtifacts")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")
	cosiFilePath := filepath.Join(testTempDir, "image.cosi")
	originalConfigFile := filepath.Join(testDir, artifactsOutputVerityConfigFile(t, baseImageInfo))
	configFile := filepath.Join(testTempDir, "artifacts-output-verity.yaml")
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

	espFiles := verifyAndSignOutputtedArtifacts(t, outputArtifactsDir, true)

	// Inject artifacts into image.
	options := InjectFilesOptions{
		BuildDir:          buildDir,
		InputImageFile:    outImageFilePath,
		OutputImageFile:   cosiFilePath,
		OutputImageFormat: "cosi",
	}
	err = InjectFilesWithConfigFile(t.Context(), injectConfigPath, options)
	if !assert.NoError(t, err) {
		return
	}

	expectedCosiMetadata := MetadataJson{
		Disk: Disk{
			Size:       5639 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 2, 3, 4, 5}),
		},
		Images: []FileSystem{
			{
				Image: ImageFile{
					Path: "images/image_1.raw.zst",
				},
				MountPoint: "/boot/efi",
				FsType:     "vfat",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeESP],
			},
			{
				Image: ImageFile{
					Path: "images/image_2.raw.zst",
				},
				MountPoint: "/boot",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
			},
			{
				Image: ImageFile{
					Path: "images/image_3.raw.zst",
				},
				MountPoint: "/",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
				Verity: &VerityConfig{
					Image: ImageFile{
						Path: "images/image_4.raw.zst",
					},
				},
			},
			{
				Image: ImageFile{
					Path: "images/image_5.raw.zst",
				},
				MountPoint: "/var",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
			},
		},
		Bootloader: CosiBootloader{
			Type: "systemd-boot",
			SystemdBoot: &SystemDBoot{
				Entries: []SystemDBootEntry{
					{
						Type: "uki-standalone",
					},
				},
			},
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	// Connect to image.
	if _, ok := extractCosiAndVerifyMetadata(t, cosiFilePath, testTempDir, expectedCosiMetadata); !ok {
		return
	}

	gptPath := filepath.Join(testTempDir, "image_gpt.raw")
	espPartitionPath := filepath.Join(testTempDir, "image_1.raw")
	bootPartitionPath := filepath.Join(testTempDir, "image_2.raw")
	rootPartitionPath := filepath.Join(testTempDir, "image_3.raw")
	rootHashPartitionPath := filepath.Join(testTempDir, "image_4.raw")
	varPartitionPath := filepath.Join(testTempDir, "image_5.raw")

	gptStat, err := os.Stat(gptPath)
	assert.NoError(t, err)

	espStat, err := os.Stat(espPartitionPath)
	assert.NoError(t, err)

	bootStat, err := os.Stat(bootPartitionPath)
	assert.NoError(t, err)

	rootStat, err := os.Stat(rootPartitionPath)
	assert.NoError(t, err)

	rootHashStat, err := os.Stat(rootHashPartitionPath)
	assert.NoError(t, err)

	varStat, err := os.Stat(varPartitionPath)
	assert.NoError(t, err)

	// Check partition sizes.
	// Standard GPT size = MBR (512) + GPT Header (512) + Partition Entries (128 × 128 = 16384) = 17408 bytes
	assert.Equal(t, int64(17408), gptStat.Size())
	assert.Equal(t, int64(500*diskutils.MiB), espStat.Size())
	assert.Equal(t, int64(2*diskutils.GiB), rootStat.Size())
	assert.Equal(t, int64(17*diskutils.MiB), rootHashStat.Size())

	// These partitions are shrunk. Their final size will vary based on base image version, package versions, filesystem
	// implementation details, and randomness. So, just enforce that the final size is below an arbitary value. Values
	// were picked by observing values seen during test and adding a good buffer.
	assert.Greater(t, int64(150*diskutils.MiB), bootStat.Size())
	assert.Greater(t, int64(150*diskutils.MiB), varStat.Size())

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

	rootUuid, _, err := shell.Execute("blkid", "--probe", "-s", "UUID", "-o", "value", rootPartitionPath)
	assert.NoError(t, err)
	rootUuid = strings.TrimSpace(rootUuid)

	rootHashUuid, _, err := shell.Execute("blkid", "--probe", "-s", "UUID", "-o", "value", rootHashPartitionPath)
	assert.NoError(t, err)
	rootHashUuid = strings.TrimSpace(rootHashUuid)

	verifyInjectedFiles(t, espMountPath, espFiles)
	verifyVerityUki(t, espMountPath, rootPartitionPath, rootHashPartitionPath, "UUID="+rootUuid, "UUID="+rootHashUuid,
		"root", buildDir, "", "restart-on-corruption", false /*inlineVerity*/)
}

func verifyAndSignOutputtedArtifacts(t *testing.T, outputArtifactsDir string, expectVerityHash bool) []string {
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
	hasBootloader := false
	hasUKI := false
	hasUKIAddon := false
	hasVerityHash := false
	espFiles := []string(nil)

	for _, entry := range injectConfig.InjectFiles {
		// Verify the type field is set
		assert.NotEmpty(t, entry.Type, "Expected type field to be set for entry with destination: %s", entry.Destination)

		switch entry.Type {
		case imagecustomizerapi.OutputArtifactsItemShim:
			assert.True(t, strings.HasPrefix(entry.Destination, "/EFI/BOOT/boot"), "Expected shim destination to start with /EFI/BOOT/boot")
			assert.True(t, strings.HasSuffix(entry.Destination, ".efi"), "Expected shim destination to end with .efi")
			assert.True(t, strings.HasPrefix(entry.Source, "./shim/"), "Expected shim source to be in shim/ subdirectory")
			hasShim = true
			espFiles = append(espFiles, entry.Destination)

		case imagecustomizerapi.OutputArtifactsItemBootloader:
			assert.True(t, strings.HasPrefix(entry.Destination, "/EFI/BOOT/grub"), "Expected bootloader destination to start with /EFI/BOOT/grub")
			assert.True(t, strings.HasSuffix(entry.Destination, ".efi"), "Expected bootloader destination to end with .efi")
			assert.True(t, strings.HasPrefix(entry.Source, "./bootloader/"), "Expected bootloader source to be in bootloader/ subdirectory")
			hasBootloader = true
			espFiles = append(espFiles, entry.Destination)

		case imagecustomizerapi.OutputArtifactsItemUkis:
			assert.True(t, strings.HasPrefix(entry.Destination, "/EFI/Linux/vmlinuz"), "Expected UKI destination to start with /EFI/Linux/vmlinuz")
			assert.True(t, strings.HasSuffix(entry.Destination, ".efi"), "Expected UKI destination to end with .efi")
			assert.True(t, strings.HasPrefix(entry.Source, "./ukis/"), "Expected UKI source to be in ukis/ subdirectory")
			assert.False(t, strings.Contains(entry.Destination, ".efi.extra.d/"), "Expected main UKI files to not be in .extra.d/ directory")
			hasUKI = true
			espFiles = append(espFiles, entry.Destination)

		case imagecustomizerapi.OutputArtifactsItemUkiAddons:
			// UKI addon file validation
			assert.True(t, strings.HasPrefix(entry.Destination, "/EFI/Linux/vmlinuz"), "Expected UKI addon destination to start with /EFI/Linux/vmlinuz")
			assert.True(t, strings.Contains(entry.Destination, ".efi.extra.d/"), "Expected UKI addon destination to be in .efi.extra.d/ subdirectory")
			assert.True(t, strings.HasSuffix(entry.Destination, ".addon.efi"), "Expected UKI addon destination to end with .addon.efi")
			assert.True(t, strings.HasPrefix(entry.Source, "./ukis/"), "Expected UKI addon source to be in ukis/ subdirectory")
			assert.True(t, strings.Contains(entry.Source, ".efi.extra.d/"), "Expected UKI addon source to be in .efi.extra.d/ subdirectory")
			hasUKIAddon = true
			espFiles = append(espFiles, entry.Destination)

		case imagecustomizerapi.OutputArtifactsItemVerityHash:
			// Verify verity hash artifact properties
			assert.Equal(t, "/root.hash", entry.Destination, "Expected verity hash destination to be /root.hash")
			assert.True(t, strings.HasPrefix(entry.Source, "./verity-hash/"), "Expected verity hash source to be in verity-hash/ subdirectory")
			assert.Equal(t, "./verity-hash/root.hash", entry.Source, "Expected verity hash source filename to match destination filename")

			// Verify the hash file exists
			hashFilePath := filepath.Join(outputArtifactsDir, entry.Source)
			hashFileExists, err := file.PathExists(hashFilePath)
			assert.NoError(t, err)
			assert.True(t, hashFileExists, "Expected verity hash file to exist at %s", hashFilePath)

			// Verify the hash file has content (should be hex-encoded root hash)
			hashContent, err := os.ReadFile(hashFilePath)
			assert.NoError(t, err)
			assert.NotEmpty(t, hashContent, "Expected verity hash file to have content")

			hasVerityHash = true
		}

		// Check that the unsigned file exists at the source path
		unsignedPath := filepath.Join(outputArtifactsDir, entry.Source)
		_, err := os.Stat(unsignedPath)
		assert.NoErrorf(t, err, "failed to check if unsigned file exists ('%s')", unsignedPath)

		// Pseudo sign the file by replacing it with a signed version
		err = pseudoSignFile(unsignedPath)
		assert.NoErrorf(t, err, "pseudo sign file failed (path='%s')", unsignedPath)
	}

	// Ensure all the expected files were seen.
	expectedCount := 4 // shim, bootloader, main UKI, UKI addon
	if expectVerityHash {
		expectedCount = 5 // + verity hash
	}

	assert.Equal(t, expectedCount, len(injectConfig.InjectFiles))
	assert.True(t, hasShim, "Expected an inject entry for shim")
	assert.True(t, hasBootloader, "Expected an inject entry for bootloader")
	assert.True(t, hasUKI, "Expected at least one inject entry for main UKI")
	assert.True(t, hasUKIAddon, "Expected at least one inject entry for UKI addon")
	if expectVerityHash {
		assert.True(t, hasVerityHash, "Expected an inject entry for verity-hash")
	}

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

func pseudoSignFile(filePath string) error {
	err := appendMarker(filePath, pseudoSignedMarker)
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
