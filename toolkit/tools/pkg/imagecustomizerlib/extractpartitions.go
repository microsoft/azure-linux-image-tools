// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

var (
	// Partition extraction errors
	ErrPartitionExtractAbsolutePath      = NewImageCustomizerError("PartitionExtract:AbsolutePath", "failed to get absolute path")
	ErrPartitionExtractIntegrityCheck    = NewImageCustomizerError("PartitionExtract:IntegrityCheck", "failed to check file system integrity")
	ErrPartitionExtractStatFile          = NewImageCustomizerError("PartitionExtract:StatFile", "failed to stat file")
	ErrPartitionExtractUnsupportedFormat = NewImageCustomizerError("PartitionExtract:UnsupportedFormat", "unsupported partition format")
	ErrPartitionExtractMetadataConstruct = NewImageCustomizerError("PartitionExtract:MetadataConstruct", "failed to construct partition metadata")
	ErrPartitionExtractRemoveRawFile     = NewImageCustomizerError("PartitionExtract:RemoveRawFile", "failed to remove raw file")
	ErrPartitionExtractRemoveTempFile    = NewImageCustomizerError("PartitionExtract:RemoveTempFile", "failed to remove temp file")
	ErrPartitionExtractCopyBlockDevice   = NewImageCustomizerError("PartitionExtract:CopyBlockDevice", "failed to copy block device")
	ErrPartitionExtractCompress          = NewImageCustomizerError("PartitionExtract:Compress", "failed to compress partition")
	ErrPartitionExtractOpenFile          = NewImageCustomizerError("PartitionExtract:OpenFile", "failed to open partition file")
)

type outputPartitionMetadata struct {
	PartitionNum      int    `json:"partitionnum"`     // Example: 1
	PartitionFilename string `json:"filename"`         // Example: image_1.raw.zst
	PartLabel         string `json:"partlabel"`        // Example: boot
	FileSystemType    string `json:"fstype"`           // Example: vfat
	PartitionTypeUuid string `json:"parttype"`         // Example: c12a7328-f81f-11d2-ba4b-00a0c93ec93b
	Uuid              string `json:"uuid"`             // Example: 4BD9-3A78
	PartUuid          string `json:"partuuid"`         // Example: 7b1367a6-5845-43f2-99b1-a742d873f590
	Mountpoint        string `json:"mountpoint"`       // Example: /mnt/os/boot
	UncompressedSize  uint64 `json:"uncompressedsize"` // Example: 104857600
}

const (
	SkippableFrameMagicNumber uint32 = 0x184D2A50
	SkippableFramePayloadSize uint32 = randomization.UuidSize
	SkippableFrameHeaderSize  int    = 8
)

// Extract all partitions of connected image into separate files with specified format.
func extractPartitions(imageLoopDevice string, outDir string, basename string, partitionFormat string,
	imageUuid [randomization.UuidSize]byte, compressionLevel int, compressionLong int,
) ([]outputPartitionMetadata, error) {
	// Get partition info
	diskPartitions, err := diskutils.GetDiskPartitions(imageLoopDevice)
	if err != nil {
		return nil, err
	}

	// Stores the output partition metadata that will be written to JSON file
	var partitionMetadataOutput []outputPartitionMetadata

	// Extract partitions to files
	for _, partition := range diskPartitions {
		if partition.Type != "part" {
			continue
		}

		partitionNum, err := getPartitionNum(partition.Path)
		if err != nil {
			return nil, err
		}

		partitionFilename := basename + "_" + strconv.Itoa(partitionNum)
		rawFilename := partitionFilename + ".raw"

		partitionFilepath, err := copyBlockDeviceToFile(outDir, partition.Path, rawFilename)
		if err != nil {
			return nil, err
		}

		partitionFullFilePath, err := filepath.Abs(partitionFilepath)
		if err != nil {
			return nil, fmt.Errorf("%w (path='%s'):\n%w", ErrPartitionExtractAbsolutePath, partitionFilepath, err)
		}

		// Sanity check the partition file.
		err = checkFileSystemFile(partition.FileSystemType, partitionFullFilePath)
		if err != nil {
			return nil, fmt.Errorf("%w (path='%s'):\n%w", ErrPartitionExtractIntegrityCheck, partitionFilepath, err)
		}

		// Get uncompressed size for raw files
		var uncompressedPartitionFileSize uint64
		stat, err := os.Stat(partitionFullFilePath)
		if err != nil {
			return nil, fmt.Errorf("%w (file='%s'):\n%w", ErrPartitionExtractStatFile, partitionFilepath, err)
		}
		uncompressedPartitionFileSize = uint64(stat.Size())

		switch partitionFormat {
		case "raw":
			// Do nothing for "raw" case
		case "raw-zst":
			partitionFilepath, err = extractRawZstPartition(partitionFilepath, imageUuid, partitionFilename, outDir,
				compressionLevel, compressionLong)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w (format='%s', supported: raw, raw-zst)", ErrPartitionExtractUnsupportedFormat, partitionFormat)
		}

		partitionMetadata, err := constructOutputPartitionMetadata(partition, partitionNum, partitionFilepath)
		if err != nil {
			return nil, fmt.Errorf("%w (partition=%d, file='%s'):\n%w", ErrPartitionExtractMetadataConstruct, partitionNum, partitionFilepath, err)
		}
		partitionMetadata.UncompressedSize = uncompressedPartitionFileSize
		partitionMetadataOutput = append(partitionMetadataOutput, partitionMetadata)
		logger.Log.Infof("Partition file created: %s", partitionFilepath)
	}

	return partitionMetadataOutput, nil
}

// Extract raw-zst partition.
func extractRawZstPartition(partitionRawFilepath string, skippableFrameMetadata [SkippableFramePayloadSize]byte,
	partitionFilename string, outDir string, compressionLevel int, compressionLong int,
) (partitionFilepath string, err error) {
	// Define file path for temporary partition
	tempPartitionFilepath := outDir + "/" + partitionFilename + "_temp.raw.zst"
	// Compress raw partition with zstd
	err = compressWithZstd(partitionRawFilepath, tempPartitionFilepath, compressionLevel, compressionLong)
	if err != nil {
		return "", err
	}
	// Remove raw file since output partition format is raw-zst
	err = os.Remove(partitionRawFilepath)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrPartitionExtractRemoveRawFile, partitionRawFilepath, err)
	}
	// Create a skippable frame containing the metadata and prepend the frame to the partition file
	partitionFilepath, err = addSkippableFrame(tempPartitionFilepath, skippableFrameMetadata, partitionFilename, outDir)
	if err != nil {
		return "", err
	}
	// Remove temp partition file
	err = os.Remove(tempPartitionFilepath)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrPartitionExtractRemoveTempFile, tempPartitionFilepath, err)
	}
	return partitionFilepath, nil
}

// Creates .raw file for the mentioned partition path.
func copyBlockDeviceToFile(outDir, devicePath, name string) (filename string, err error) {
	const (
		defaultBlockSize = 1024 * 1024 // 1MB
		squashErrors     = true
	)

	fullPath := filepath.Join(outDir, name)
	ddArgs := []string{
		fmt.Sprintf("if=%s", devicePath),       // Input file.
		fmt.Sprintf("of=%s", fullPath),         // Output file.
		fmt.Sprintf("bs=%d", defaultBlockSize), // Size of one copied block.
		"conv=sparse",
	}

	err = shell.ExecuteLive(squashErrors, "dd", ddArgs...)
	if err != nil {
		return "", fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrPartitionExtractCopyBlockDevice, devicePath, fullPath, err)
	}

	return fullPath, nil
}

// Compress file from .raw to .raw.zst format using zstd.
func compressWithZstd(partitionRawFilepath string, outputPartitionFilepath string,
	compressionLevel int, compressionLong int,
) (err error) {
	args := buildZstdArgs(partitionRawFilepath, outputPartitionFilepath, compressionLevel, compressionLong)
	err = shell.ExecuteLive(true, "zstd", args...)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrPartitionExtractCompress, partitionRawFilepath, err)
	}

	return nil
}

func buildZstdArgs(inputFile string, outputFile string, compressionLevel int, compressionLong int) []string {
	args := []string{"--force"} // Overwrite a file with same name if it exists.

	if compressionLevel >= imagecustomizerapi.UltraCosiCompressionThreshold {
		args = append(args, "--ultra") // Needed for the highest compression levels.
	}

	args = append(args, fmt.Sprintf("-%d", compressionLevel))      // Configure the compression level.
	args = append(args, fmt.Sprintf("--long=%d", compressionLong)) // Configure the long-range matching window size.
	args = append(args, "-T0")                                     // Use all available threads.
	args = append(args, inputFile)
	args = append(args, "-o", outputFile)

	return args
}

// Prepend a skippable frame with the metadata to the specified partition file.
func addSkippableFrame(tempPartitionFilepath string, skippableFrameMetadata [SkippableFramePayloadSize]byte, partitionFilename string, outDir string) (partitionFilepath string, err error) {
	// Open tempPartitionFile for reading
	tempPartitionFile, err := os.OpenFile(tempPartitionFilepath, os.O_RDWR, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrPartitionExtractOpenFile, tempPartitionFilepath, err)
	}
	// Create a skippable frame
	skippableFrame := createSkippableFrame(SkippableFrameMagicNumber, SkippableFramePayloadSize, skippableFrameMetadata)
	// Define the final partition file path
	partitionFilepath = outDir + "/" + partitionFilename + ".raw.zst"
	// Create partition file
	finalFile, err := os.Create(partitionFilepath)
	if err != nil {
		return "", err
	}
	// Write the skippable frame to file
	_, err = finalFile.Write(skippableFrame)
	if err != nil {
		return "", err
	}
	// Copy the data from the tempPartitionFile into finalFile
	_, err = io.Copy(finalFile, tempPartitionFile)
	if err != nil {
		return "", err
	}
	return partitionFilepath, nil
}

// Creates a skippable frame.
func createSkippableFrame(magicNumber uint32, frameSize uint32, skippableFrameMetadata [SkippableFramePayloadSize]byte) (skippableFrame []byte) {
	// Calculate the length of the byte array
	lengthOfByteArray := SkippableFrameHeaderSize + len(skippableFrameMetadata)
	// Define the Skippable frame
	skippableFrame = make([]byte, lengthOfByteArray)
	// Magic_Number
	binary.LittleEndian.PutUint32(skippableFrame, magicNumber)
	// Frame_Size
	binary.LittleEndian.PutUint32(skippableFrame[4:8], frameSize)
	// User_Data
	copy(skippableFrame[8:8+frameSize], skippableFrameMetadata[:])

	logger.Log.Infof("Skippable frame has been created with the following metadata: %d", skippableFrame[8:8+frameSize])

	return skippableFrame
}

// Construct outputPartitionMetadata for given partition.
func constructOutputPartitionMetadata(diskPartition diskutils.PartitionInfo, partitionNum int, partitionFilepath string) (partitionMetadata outputPartitionMetadata, err error) {
	partitionMetadata.PartitionNum = partitionNum
	partitionMetadata.PartitionFilename = filepath.Base(partitionFilepath)
	partitionMetadata.PartLabel = diskPartition.PartLabel
	partitionMetadata.FileSystemType = diskPartition.FileSystemType
	partitionMetadata.PartitionTypeUuid = diskPartition.PartitionTypeUuid
	partitionMetadata.Uuid = diskPartition.Uuid
	partitionMetadata.PartUuid = diskPartition.PartUuid
	partitionMetadata.Mountpoint = diskPartition.Mountpoint

	return partitionMetadata, nil
}
