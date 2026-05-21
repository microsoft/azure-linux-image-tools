package imagecustomizerlib

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

var (
	expectedCosiMetadataForAzlCoreEfi = MetadataJson{
		Disk: Disk{
			Size:       4 * diskutils.GiB,
			GptRegions: newTestCosiGptSections([]int{1, 2}),
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
				MountPoint: "/",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
			},
		},
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	// Azure Linux 4.0 core-efi base image has a 12 GiB virtual disk and tags the
	// rootfs with the discoverable-partitions arch-specific root GUID instead of
	// the legacy linux-filesystem GUID used by AzL2/AzL3.
	expectedCosiMetadataForAzl4CoreEfi = MetadataJson{
		Disk: Disk{
			Size:       12 * diskutils.GiB,
			GptRegions: newTestCosiGptSections([]int{1, 2}),
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
				MountPoint: "/",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeRoot],
			},
		},
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	expectedCosiFileSystemsForUbuntu2204 = []FileSystem{
		{
			Image: ImageFile{
				Path: "images/image_1.raw.zst",
			},
			MountPoint: "/",
			FsType:     "ext4",
			PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
		},
		{
			Image: ImageFile{
				Path: "images/image_15.raw.zst",
			},
			MountPoint: "/boot/efi",
			FsType:     "vfat",
			PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeESP],
		},
	}

	expectedCosiMetadataForUbuntu2204CloudAmd64 = MetadataJson{
		Disk: Disk{
			Size:       30721 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 14, 15}),
		},
		Images: expectedCosiFileSystemsForUbuntu2204,
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	expectedCosiMetadataForUbuntu2204CloudArm64 = MetadataJson{
		Disk: Disk{
			Size:       30721 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 15}),
		},
		Images: expectedCosiFileSystemsForUbuntu2204,
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	expectedCosiFileSystemsForUbuntu2404 = []FileSystem{
		{
			Image: ImageFile{
				Path: "images/image_1.raw.zst",
			},
			MountPoint: "/",
			FsType:     "ext4",
			PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
		},
		{
			Image: ImageFile{
				Path: "images/image_15.raw.zst",
			},
			MountPoint: "/boot/efi",
			FsType:     "vfat",
			PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeESP],
		},
		{
			Image: ImageFile{
				Path: "images/image_16.raw.zst",
			},
			MountPoint: "/boot",
			FsType:     "ext4",
			PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeXbootldr],
		},
	}

	expectedCosiMetadataForUbuntu2404CloudAmd64 = MetadataJson{
		Disk: Disk{
			Size:       30721 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 14, 15, 16}),
		},
		Images: expectedCosiFileSystemsForUbuntu2404,
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	expectedCosiMetadataForUbuntu2404CloudArm64 = MetadataJson{
		Disk: Disk{
			Size:       30721 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 15, 16}),
		},
		Images: expectedCosiFileSystemsForUbuntu2404,
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}
)

// expectedCosiMetadataForAzureLinux returns the expected COSI metadata for the given Azure Linux core-efi base image.
// AzL4 differs from AzL2/AzL3 in disk size and rootfs partition type GUID.
func expectedCosiMetadataForAzureLinux(baseImageInfo testBaseImageInfo) (MetadataJson, error) {
	switch baseImageInfo.Version {
	case baseImageVersionAzl2, baseImageVersionAzl3:
		return expectedCosiMetadataForAzlCoreEfi, nil
	case baseImageVersionAzl4:
		return expectedCosiMetadataForAzl4CoreEfi, nil
	default:
		return MetadataJson{}, fmt.Errorf("unexpected Azure Linux version: %s", baseImageInfo.Version)
	}
}

func TestAddSkippableFrame(t *testing.T) {
	// Create a skippable frame containing the metadata and prepend the frame to the partition file
	skippableFrameMetadata, _, err := randomization.CreateUuid()
	assert.NoError(t, err)

	// Create test raw partition file
	partitionFilename := "test"
	partitionRawFilepath, err := createTestRawPartitionFile(partitionFilename)
	assert.NoError(t, err)

	// Compress to .raw.zst partition file
	tempPartitionFilepath := testDir + partitionFilename + "_temp.raw.zst"
	err = compressWithZstd(partitionRawFilepath, tempPartitionFilepath, imagecustomizerapi.DefaultCosiCompressionLevel,
		imagecustomizerapi.DefaultCosiCompressionLong)
	assert.NoError(t, err)

	// Test adding the skippable frame
	partitionFilepath, err := addSkippableFrame(tempPartitionFilepath, skippableFrameMetadata, partitionFilename, testDir)
	assert.NoError(t, err)

	// Verify decompression with skippable frame
	err = verifySkippableFrameDecompression(partitionRawFilepath, partitionFilepath)
	assert.NoError(t, err)

	// Verify skippable frame metadata
	err = verifySkippableFrameMetadataFromFile(partitionFilepath, SkippableFrameMagicNumber, SkippableFramePayloadSize, skippableFrameMetadata)
	assert.NoError(t, err)

	// Remove test partition files
	err = os.Remove(partitionRawFilepath)
	assert.NoError(t, err)
	err = os.Remove(tempPartitionFilepath)
	assert.NoError(t, err)
	err = os.Remove(partitionFilepath)
	assert.NoError(t, err)
}

func createTestRawPartitionFile(filename string) (string, error) {
	// Test data
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	// Output file name
	outputFilename := filename + ".raw"

	// Write data to file
	err := os.WriteFile(outputFilename, testData, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to write test data to partition file %s:\n%w", filename, err)
	}
	logger.Log.Infof("Test raw partition file created: %s", outputFilename)
	return outputFilename, nil
}

func extractZstFile(zstFilePath string, outputFilePath string) error {
	err := shell.ExecuteLive(true, "zstd", "-d", zstFilePath, "-o", outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to decompress %s with zstd:\n%w", zstFilePath, err)
	}

	return nil
}

func extractCosiAndVerifyMetadata(t *testing.T, cosiFilePath string, partitionsOutputDir string,
	expectedMetadata MetadataJson,
) (MetadataJson, bool) {
	partitionsPaths, metadata, err := extractCosi(cosiFilePath, partitionsOutputDir)
	if !assert.NoError(t, err) {
		return MetadataJson{}, false
	}

	assert.Equal(t, "1.2", metadata.Version)

	assert.Equal(t, expectedMetadata.Disk.Size, metadata.Disk.Size)
	assert.Equal(t, DiskTypeGpt, metadata.Disk.Type)
	assert.Equal(t, 512, metadata.Disk.LbaSize)

	assert.Equal(t, len(expectedMetadata.Disk.GptRegions), len(partitionsPaths))

	if assert.Equal(t, len(expectedMetadata.Disk.GptRegions), len(metadata.Disk.GptRegions)) {
		for i := range expectedMetadata.Disk.GptRegions {
			expectedGptRegion := expectedMetadata.Disk.GptRegions[i]
			actualGptRegion := metadata.Disk.GptRegions[i]

			verifyCosiImageFile(t, expectedGptRegion.Image, actualGptRegion.Image)
			assert.Equal(t, expectedGptRegion.Type, actualGptRegion.Type)
			assert.Equal(t, expectedGptRegion.Number, actualGptRegion.Number)
		}
	}

	if assert.Equal(t, len(expectedMetadata.Images), len(metadata.Images)) {
		for i := range expectedMetadata.Images {
			expectedImage := expectedMetadata.Images[i]
			actualImage := metadata.Images[i]

			verifyCosiImageFile(t, expectedImage.Image, actualImage.Image)
			assert.Equal(t, expectedImage.MountPoint, actualImage.MountPoint)
			assert.Equal(t, expectedImage.FsType, actualImage.FsType)
			assert.Equal(t, expectedImage.PartType, actualImage.PartType)

			assert.Equal(t, expectedImage.Verity != nil, actualImage.Verity != nil)
			if expectedImage.Verity != nil && actualImage.Verity != nil {
				verifyCosiImageFile(t, expectedImage.Verity.Image, actualImage.Verity.Image)
				assert.Regexp(t, `^[0-9a-fA-F]{64}$`, actualImage.Verity.Roothash)

				inlineVerity := expectedImage.Verity.Image.Path == expectedImage.Image.Path
				assert.Equal(t, inlineVerity, actualImage.Verity.HashOffset != nil)
			}
		}
	}

	assert.Equal(t, expectedMetadata.Bootloader.Type, metadata.Bootloader.Type)

	expectedSystemdBoot := expectedMetadata.Bootloader.SystemdBoot
	actualSystemdBoot := metadata.Bootloader.SystemdBoot
	assert.Equal(t, expectedSystemdBoot != nil, actualSystemdBoot != nil)
	if expectedSystemdBoot != nil && actualSystemdBoot != nil {
		if assert.Equal(t, len(expectedSystemdBoot.Entries), len(actualSystemdBoot.Entries)) {
			for i := range expectedSystemdBoot.Entries {
				expectedEntry := expectedSystemdBoot.Entries[i]
				actualEntry := actualSystemdBoot.Entries[i]

				assert.Equal(t, expectedEntry.Type, actualEntry.Type)

				assert.NotEqual(t, "", actualEntry.Path)
				assert.NotEqual(t, "", actualEntry.Cmdline)
				assert.NotEqual(t, "", actualEntry.Kernel)
			}
		}
	}

	assert.Equal(t, expectedMetadata.Compression, metadata.Compression)

	return metadata, true
}

func verifyCosiImageFile(t *testing.T, expected ImageFile, actual ImageFile) {
	assert.Equal(t, expected.Path, actual.Path)

	assert.Less(t, uint64(0), actual.CompressedSize)
	assert.LessOrEqual(t, actual.CompressedSize, actual.UncompressedSize)
	assert.Regexp(t, `^[0-9a-fA-F]{96}$`, actual.Sha384)
}

func extractCosi(cosiFilePath, partitionsOutputDir string) ([]string, MetadataJson, error) {
	var extractedParitionsPaths []string

	err := os.MkdirAll(partitionsOutputDir, os.ModePerm)
	if err != nil {
		return nil, MetadataJson{}, fmt.Errorf("failed to create output directory:\n%w", err)
	}

	// Open the COSI file
	cosiFile, err := os.Open(cosiFilePath)
	if err != nil {
		return nil, MetadataJson{}, fmt.Errorf("failed to open COSI file:\n%w", err)
	}
	defer cosiFile.Close()

	cosiMetadata := MetadataJson{}
	foundCosiMetadata := false

	tarReader := tar.NewReader(cosiFile)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, MetadataJson{}, fmt.Errorf("error reading tar:\n%w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Validate the file path to prevent directory traversal
		cleanPath := filepath.Clean(header.Name)
		if strings.Contains(cleanPath, "..") {
			return nil, MetadataJson{}, fmt.Errorf("invalid file path in tar archive: %s", header.Name)
		}

		switch {
		case filepath.Ext(header.Name) == ".zst":
			outputFilePath, err := writeZstAndRawToFile(partitionsOutputDir, header, tarReader)
			if err != nil {
				return nil, MetadataJson{}, fmt.Errorf("failed to extract partition from cosi:\n%w", err)
			}

			extractedParitionsPaths = append(extractedParitionsPaths, outputFilePath)
			logger.Log.Debugf("Extracted partition file: %s", outputFilePath)

		case header.Name == CosiMetadataName:
			cosiMetadata, err = readCosiMetadata(tarReader)
			if err != nil {
				return nil, MetadataJson{}, fmt.Errorf("failed to read cosi metadata:\n%w", err)
			}

			foundCosiMetadata = true
		}
	}

	if !foundCosiMetadata {
		return nil, MetadataJson{}, fmt.Errorf("no %s found in cosi file", CosiMetadataName)
	}

	return extractedParitionsPaths, cosiMetadata, nil
}

func writeZstAndRawToFile(outputDir string, header *tar.Header, tarReader io.Reader) (string, error) {
	imageFileName := filepath.Base(header.Name)

	zstFilePath := filepath.Join(outputDir, imageFileName)
	// remove the .zst extension to get the output file name
	rawImageFile := imageFileName[:len(imageFileName)-len(filepath.Ext(imageFileName))]
	outputFilePath := filepath.Join(outputDir, rawImageFile)

	// Create the .zst file
	zstFile, err := os.Create(zstFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open .zst file:\n%w", err)
	}
	defer zstFile.Close()

	// Create the output file
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file:\n%w", err)
	}
	defer outFile.Close()

	// Extract .zst file from tarball.
	_, err = io.Copy(zstFile, tarReader)
	if err != nil {
		return "", fmt.Errorf("failed to extract file from tarball:\n%w", err)
	}

	// Prepare file to be read back.
	_, err = zstFile.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("failed to seek to origin of zst file:\n%w", err)
	}

	// Create a new zstd reader
	zstReader, err := zstd.NewReader(zstFile)
	if err != nil {
		return "", fmt.Errorf("failed to create zstd reader:\n%w", err)
	}
	defer zstReader.Close()

	// Decompress the .zst file and write to the output file
	if _, err := io.Copy(outFile, zstReader); err != nil {
		return "", fmt.Errorf("failed to decompress and write to output file:\n%w", err)
	}

	return outputFilePath, nil
}

func readCosiMetadata(src io.Reader) (MetadataJson, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return MetadataJson{}, fmt.Errorf("failed to read metadata.json:\n%w", err)
	}

	metadata := MetadataJson{}
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return MetadataJson{}, fmt.Errorf("failed to parse metadata.json:\n%w", err)
	}

	return metadata, nil
}

// Decompress the .raw.zst partition file and verify the hash matches with the source .raw file
func verifySkippableFrameDecompression(rawPartitionFilepath string, rawZstPartitionFilepath string) (err error) {
	// Decompressing .raw.zst file
	decompressedPartitionFilepath := "decompressed.raw"
	err = extractZstFile(rawZstPartitionFilepath, decompressedPartitionFilepath)
	if err != nil {
		return err
	}

	// Calculating hashes
	rawPartitionFileHash, err := file.GenerateSHA256(rawPartitionFilepath)
	if err != nil {
		return fmt.Errorf("error generating SHA256:\n%w", err)
	}
	decompressedPartitionFileHash, err := file.GenerateSHA256(decompressedPartitionFilepath)
	if err != nil {
		return fmt.Errorf("error generating SHA256:\n%w", err)
	}

	// Verifying hashes are equal
	if rawPartitionFileHash != decompressedPartitionFileHash {
		return fmt.Errorf("decompressed partition file hash does not match source partition file hash: %s != %s", decompressedPartitionFileHash, rawPartitionFilepath)
	}
	logger.Log.Debugf("Decompressed partition file hash matches source partition file hash!")

	// Removing decompressed file
	err = os.Remove(decompressedPartitionFilepath)
	if err != nil {
		return fmt.Errorf("failed to remove raw file %s:\n%w", decompressedPartitionFilepath, err)
	}

	return nil
}

// Verifies that the skippable frame has been correctly prepended to the partition file with the correct data
func verifySkippableFrameMetadataFromFile(partitionFilepath string, magicNumber uint32, frameSize uint32, skippableFrameMetadata [SkippableFramePayloadSize]byte) (err error) {
	// Read existing data from the partition file
	existingData, err := os.ReadFile(partitionFilepath)
	if err != nil {
		return fmt.Errorf("failed to read partition file %s:\n%w", partitionFilepath, err)
	}

	// Verify that the skippable frame has been prepended to the partition file by validating magicNumber
	if binary.LittleEndian.Uint32(existingData[0:4]) != magicNumber {
		return fmt.Errorf("skippable frame has not been prepended to the partition file:\n %d != %d", binary.LittleEndian.Uint32(existingData[0:4]), magicNumber)
	}
	logger.Log.Infof("Skippable frame had been correctly prepended to the partition file.")

	// Verify that the skippable frame has the correct frame size by validating frameSize
	if binary.LittleEndian.Uint32(existingData[4:8]) != frameSize {
		return fmt.Errorf("skippable frame frameSize field does not match the defined frameSize:\n %d != %d", binary.LittleEndian.Uint32(existingData[4:8]), frameSize)
	}
	logger.Log.Infof("Skippable frame frameSize field is correct.")

	// Verify that the skippable frame has the correct inserted metadata by validating skippableFrameMetadata
	if !bytes.Equal(existingData[8:8+frameSize], skippableFrameMetadata[:]) {
		return fmt.Errorf("skippable frame metadata does not match the inserted metadata:\n %d != %d", existingData[8:8+frameSize], skippableFrameMetadata[:])
	}
	logger.Log.Infof("Skippable frame is valid and contains the correct metadata!")

	return nil
}

// Tests partition extracting with partition resize enabled, but where the partition resize is a no-op.
func TestCustomizeImageNopShrink(t *testing.T) {
	var err error

	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageNopShrink")
	defer os.RemoveAll(testTempDir)

	configFile := filepath.Join(testDir, "consume-space.yaml")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.cosi")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "cosi", true, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Attach partition files.
	expectedCosiMetadata, err := expectedCosiMetadataForAzureLinux(baseImageInfo)
	assert.NoError(t, err)
	if _, ok := extractCosiAndVerifyMetadata(t, outImageFilePath, testTempDir, expectedCosiMetadata); !ok {
		return
	}

	espPartitionNumber := 1
	rootfsPartitionNumber := 2

	espPartitionZstFilePath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw.zst", espPartitionNumber))
	rootfsPartitionZstFilePath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw.zst", rootfsPartitionNumber))

	espPartitionFilePath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", espPartitionNumber))
	rootfsPartitionFilePath := filepath.Join(testTempDir, fmt.Sprintf("image_%d.raw", rootfsPartitionNumber))

	// Check the file type of the output files.
	checkFileType(t, espPartitionZstFilePath, "zst")
	checkFileType(t, rootfsPartitionZstFilePath, "zst")

	// Mount the partitions.
	mountsDir := filepath.Join(testTempDir, "testmounts")
	espMountDir := filepath.Join(mountsDir, "esp")
	rootfsMountDir := filepath.Join(mountsDir, "rootfs")

	espLoopback, err := safeloopback.NewLoopback(espPartitionFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer espLoopback.Close()

	rootfsLoopback, err := safeloopback.NewLoopback(rootfsPartitionFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer rootfsLoopback.Close()

	espMount, err := safemount.NewMount(espLoopback.DevicePath(), espMountDir, "vfat", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer espMount.Close()

	rootfsMount, err := safemount.NewMount(rootfsLoopback.DevicePath(), rootfsMountDir, "ext4", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer rootfsMount.Close()

	// Get the file sizes.
	var rootfsStat unix.Statfs_t
	err = unix.Statfs(rootfsMountDir, &rootfsStat)
	if !assert.NoError(t, err) {
		return
	}

	bigFileStat, err := os.Stat(filepath.Join(rootfsMountDir, "bigfile"))
	if !assert.NoError(t, err) {
		return
	}

	rootfsZstFileStat, err := os.Stat(rootfsPartitionZstFilePath)
	if !assert.NoError(t, err) {
		return
	}

	// Confirm that there is almost 0 free space left, thus preventing the shrink partition operation from doing
	// anything.
	rootfsFreeSpace := int64(rootfsStat.Bfree) * rootfsStat.Frsize
	assert.LessOrEqual(t, rootfsFreeSpace, int64(32*diskutils.MiB), "check rootfs free space")

	// Ensure that zst succesfully compressed the rootfs partition.
	// In particular, bigfile, which is all 0s, should compress down to basically nothing.
	rootfsSizeLessBigFile := int64(rootfsStat.Blocks)*rootfsStat.Frsize - bigFileStat.Size()
	assert.LessOrEqual(t, rootfsZstFileStat.Size(), rootfsSizeLessBigFile, "check compression size")
}

func TestCustomizeImageExtractEmptyPartition(t *testing.T) {
	var err error

	baseImage, _ := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageExtractEmptyPartition")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	configFile := filepath.Join(testDir, "partitions-unformatted-partition.yaml")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "cosi", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	expectedCosiMetadata := MetadataJson{
		Disk: Disk{
			Size:       4130 * diskutils.MiB,
			GptRegions: newTestCosiGptSections([]int{1, 2, 3}),
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
				MountPoint: "/",
				FsType:     "ext4",
				PartType:   imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeLinuxGeneric],
			},
		},
		Bootloader: CosiBootloader{
			Type: "grub",
		},
		Compression: Compression{
			MaxWindowLog: imagecustomizerapi.DefaultCosiCompressionLong,
		},
	}

	// Attach partition files.
	if _, ok := extractCosiAndVerifyMetadata(t, outImageFilePath, buildDir, expectedCosiMetadata); !ok {
		return
	}

	espPartitionNumber := 1
	rootfsPartitionNumber := 2

	espPartitionFilePath := filepath.Join(buildDir, fmt.Sprintf("image_%d.raw", espPartitionNumber))
	rootfsPartitionFilePath := filepath.Join(buildDir, fmt.Sprintf("image_%d.raw", rootfsPartitionNumber))

	// Mount the partitions.
	mountsDir := filepath.Join(buildDir, "testmounts")
	espMountDir := filepath.Join(mountsDir, "esp")
	rootfsMountDir := filepath.Join(mountsDir, "rootfs")

	espLoopback, err := safeloopback.NewLoopback(espPartitionFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer espLoopback.Close()

	rootfsLoopback, err := safeloopback.NewLoopback(rootfsPartitionFilePath)
	if !assert.NoError(t, err) {
		return
	}
	defer rootfsLoopback.Close()

	espMount, err := safemount.NewMount(espLoopback.DevicePath(), espMountDir, "vfat", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer espMount.Close()

	rootfsMount, err := safemount.NewMount(rootfsLoopback.DevicePath(), rootfsMountDir, "ext4", 0, "", true)
	if !assert.NoError(t, err) {
		return
	}
	defer rootfsMount.Close()
}

// Ensure that Image Customizer doesn't fail if the user modifies the /etc/fstab in a postCustomization script.
func TestCustomizeImageFstabDelete(t *testing.T) {
	var err error

	baseImage, baseImageInfo := checkSkipForCustomizeDefaultAzureLinuxImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageFstabDelete")
	defer os.RemoveAll(testTempDir)

	buildDir := filepath.Join(testTempDir, "build")
	configFile := filepath.Join(testDir, "fstab-delete.yaml")
	outImageFilePath := filepath.Join(testTempDir, "image.cosi")

	// Customize image.
	// Ensure there is no error even though the /etc/fstab file was deleted.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "cosi",
		false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	expectedCosiMetadata, err := expectedCosiMetadataForAzureLinux(baseImageInfo)
	assert.NoError(t, err)
	if _, ok := extractCosiAndVerifyMetadata(t, outImageFilePath, buildDir, expectedCosiMetadata); !ok {
		return
	}
}

func TestBuildZstdArgs_UltraLevel(t *testing.T) {
	testCases := []struct {
		name     string
		level    int
		expected []string
	}{
		{"level 18", 18, []string{"--force", "-18", "--long=27", "-T0", "in.raw", "-o", "out.raw.zst"}},
		{"level 19", 19, []string{"--force", "-19", "--long=27", "-T0", "in.raw", "-o", "out.raw.zst"}},
		{"level 20", 20, []string{"--force", "--ultra", "-20", "--long=27", "-T0", "in.raw", "-o", "out.raw.zst"}},
		{"level 21", 21, []string{"--force", "--ultra", "-21", "--long=27", "-T0", "in.raw", "-o", "out.raw.zst"}},
		{"level 22", 22, []string{"--force", "--ultra", "-22", "--long=27", "-T0", "in.raw", "-o", "out.raw.zst"}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := buildZstdArgs("in.raw", "out.raw.zst", tc.level, imagecustomizerapi.DefaultCosiCompressionLong)
			assert.Equal(t, tc.expected, args)
		})
	}
}

func newTestCosiGptSections(partNums []int) []GptDiskRegion {
	gptRegions := []GptDiskRegion{
		{
			Image: ImageFile{
				Path: "images/image_gpt.raw.zst",
			},
			Type: "primary-gpt",
		},
	}

	for _, partNum := range partNums {
		partition := GptDiskRegion{
			Image: ImageFile{
				Path: fmt.Sprintf("images/image_%d.raw.zst", partNum),
			},
			Type:   "partition",
			Number: ptrutils.PtrTo(partNum),
		}
		gptRegions = append(gptRegions, partition)
	}

	return gptRegions
}
