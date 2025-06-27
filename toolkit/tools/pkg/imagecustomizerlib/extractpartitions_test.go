package imagecustomizerlib

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

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
	err = compressWithZstd(partitionRawFilepath, tempPartitionFilepath)
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

// extractPartitionsFromCosi extracts the partition files from a COSI file
// and returns a list of their paths.
// It skips directories, metadata.json, and files that are not .zst.
// It also decompresses the .zst files and writes them to the output directory.
func extractPartitionsFromCosi(cosiFilePath, outputDir string) ([]string, error) {
	var extractedParitionsPaths []string

	// Open the COSI file
	cosiFile, err := os.Open(cosiFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open COSI file: %w", err)
	}
	defer cosiFile.Close()

	tarReader := tar.NewReader(cosiFile)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}
		// Skip metadata.json
		if header.Name == "metadata.json" {
			continue
		}
		// Skip files that are not .zst
		if filepath.Ext(header.Name) != ".zst" {
			continue
		}

		// Validate the file path to prevent directory traversal
		cleanPath := filepath.Clean(header.Name)
		if strings.Contains(cleanPath, "..") {
			return nil, fmt.Errorf("invalid file path in tar archive: %s", header.Name)
		}

		imageFileName := filepath.Base(header.Name)

		zstFilePath := filepath.Join(outputDir, imageFileName)
		// remove the .zst extension to get the output file name
		rawImageFile := imageFileName[:len(imageFileName)-len(filepath.Ext(imageFileName))]
		outputFilePath := filepath.Join(outputDir, rawImageFile)
		outputDir := filepath.Dir(outputFilePath)
		err = os.MkdirAll(outputDir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		// Create the .zst file
		zstFile, err := os.Create(zstFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open .zst file: %w", err)
		}
		defer zstFile.Close()

		// Create the output file
		outFile, err := os.Create(outputFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		// Extract .zst file from tarball.
		_, err = io.Copy(zstFile, tarReader)
		if err != nil {
			return nil, fmt.Errorf("failed to extract file from tarball: %w", err)
		}

		// Prepare file to be read back.
		_, err = zstFile.Seek(0, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to origin of zst file: %w", err)
		}

		// Create a new zstd reader
		zstReader, err := zstd.NewReader(zstFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd reader: %w", err)
		}
		defer zstReader.Close()

		// Decompress the .zst file and write to the output file
		if _, err := io.Copy(outFile, zstReader); err != nil {
			return nil, fmt.Errorf("failed to decompress and write to output file: %w", err)
		}

		extractedParitionsPaths = append(extractedParitionsPaths, outputFilePath)
		logger.Log.Debugf("Extracted partition file: %s", outputFilePath)

	}

	return extractedParitionsPaths, nil
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

	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	configFile := filepath.Join(testDir, "consume-space.yaml")
	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageNopShrink")
	buildDir := filepath.Join(testTempDir, "build")
	outImageFilePath := filepath.Join(testTempDir, "image.cosi")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "cosi", true, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Attach partition files.
	partitionsPaths, err := extractPartitionsFromCosi(outImageFilePath, testTempDir)

	if !assert.NoError(t, err) || !assert.Len(t, partitionsPaths, 2) {
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

	baseImage, _ := checkSkipForCustomizeDefaultImage(t)

	testTempDir := filepath.Join(tmpDir, "TestCustomizeImageExtractEmptyPartition")
	buildDir := filepath.Join(testTempDir, "build")
	configFile := filepath.Join(testDir, "partitions-unformatted-partition.yaml")
	outImageFilePath := filepath.Join(testTempDir, "image.raw")

	// Customize image.
	err = CustomizeImageWithConfigFile(t.Context(), buildDir, configFile, baseImage, nil, outImageFilePath, "cosi", false /*useBaseImageRpmRepos*/, "" /*packageSnapshotTime*/)
	if !assert.NoError(t, err) {
		return
	}

	// Attach partition files.
	partitionsPaths, err := extractPartitionsFromCosi(outImageFilePath, buildDir)

	if !assert.NoError(t, err) || !assert.Len(t, partitionsPaths, 2) {
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
	defer espLoopback.Close()

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
