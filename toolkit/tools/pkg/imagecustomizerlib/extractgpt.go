// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

var (
	// GPT extraction errors
	ErrGptExtractReadTable      = NewImageCustomizerError("GptExtract:ReadTable", "failed to read partition table")
	ErrGptExtractNoTable        = NewImageCustomizerError("GptExtract:NoTable", "no partition table found")
	ErrGptExtractPrimary        = NewImageCustomizerError("GptExtract:Primary", "failed to extract primary GPT")
	ErrGptExtractCompress       = NewImageCustomizerError("GptExtract:Compress", "failed to compress GPT data")
	ErrGptExtractUnsupported    = NewImageCustomizerError("GptExtract:Unsupported", "unsupported partition table type")
	ErrGptExtractRemoveTempFile = NewImageCustomizerError("GptExtract:RemoveTempFile", "failed to remove temporary file")
)

const (
	// GPT structure constants (for 512-byte sectors)
	// LBA 0: Protective MBR (1 sector)
	// LBA 1: GPT Header (1 sector)
	// LBA 2-33: Partition Entry Array (32 sectors, 16384 bytes minimum)
	// Total primary GPT: 34 sectors

	gptPrimaryEntriesLbaStart = 2  // Partition entries start at LBA 2
	gptPartitionEntryCount    = 32 // 32 sectors for partition entries (512 * 32 = 16384)
)

// GptExtractedData holds the extracted GPT information and file paths
type GptExtractedData struct {
	CompressedFilePath string                    // Path to the compressed GPT file (e.g., gpt.raw.zst)
	UncompressedSize   uint64                    // Size of uncompressed GPT data
	PartitionTable     *diskutils.PartitionTable // Parsed partition table information
	DiskSize           uint64                    // Total disk size in bytes
}

// extractGptData extracts the GPT/MBR partition table data from a disk image.
// For GPT disks, this extracts:
//   - Protective MBR (LBA 0)
//   - Primary GPT Header (LBA 1)
//   - Primary Partition Entry Array (LBA 2-33 for 512-byte sectors)
//
// For MBR disks, this extracts:
//   - MBR (first sector only)
func extractGptData(diskDevPath string, rawImageFile string, outDir string, basename string,
	imageUuid [randomization.UuidSize]byte, compressionLevel int, compressionLong int,
) (*GptExtractedData, error) {
	// Read the partition table to get metadata
	partitionTable, err := diskutils.ReadDiskPartitionTable(diskDevPath)
	if err != nil {
		return nil, fmt.Errorf("%w (device='%s'):\n%w", ErrGptExtractReadTable, diskDevPath, err)
	}

	if partitionTable == nil {
		return nil, fmt.Errorf("%w (device='%s')", ErrGptExtractNoTable, diskDevPath)
	}

	logger.Log.Infof("Detected partition table type: %s", partitionTable.Label)

	var rawGptFilePath string
	var uncompressedSize uint64

	switch partitionTable.Label {
	case "gpt":
		rawGptFilePath, uncompressedSize, err = extractGptTableData(diskDevPath, outDir, basename, partitionTable)
		if err != nil {
			return nil, err
		}

	case "dos":
		// MBR - extract just the first sector
		rawGptFilePath, uncompressedSize, err = extractMbrData(diskDevPath, outDir, basename, partitionTable)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("%w (type='%s')", ErrGptExtractUnsupported, partitionTable.Label)
	}

	// Compress the GPT data
	gptFilename := basename + "_gpt"
	compressedFilePath, err := compressGptData(rawGptFilePath, outDir, gptFilename, imageUuid,
		compressionLevel, compressionLong)
	if err != nil {
		return nil, err
	}

	// Clean up raw file
	err = os.Remove(rawGptFilePath)
	if err != nil {
		return nil, fmt.Errorf("%w (file='%s'):\n%w", ErrGptExtractRemoveTempFile, rawGptFilePath, err)
	}

	logger.Log.Infof("GPT data extracted and compressed: %s (uncompressed size: %d bytes)",
		compressedFilePath, uncompressedSize)

	// Get the disk size from the raw image file
	diskSize, err := getDiskSize(rawImageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk size:\n%w", err)
	}

	return &GptExtractedData{
		CompressedFilePath: compressedFilePath,
		UncompressedSize:   uncompressedSize,
		PartitionTable:     partitionTable,
		DiskSize:           diskSize,
	}, nil
}

// getDiskSize returns the size of a disk image file in bytes
func getDiskSize(rawImageFile string) (uint64, error) {
	fileInfo, err := os.Stat(rawImageFile)
	if err != nil {
		return 0, fmt.Errorf("failed to stat disk image file (%s):\n%w", rawImageFile, err)
	}
	return uint64(fileInfo.Size()), nil
}

// extractGptTableData extracts the primary GPT structure
func extractGptTableData(diskDevPath string, outDir string, basename string,
	partitionTable *diskutils.PartitionTable,
) (string, uint64, error) {
	sectorSize := partitionTable.SectorSize
	if sectorSize == 0 {
		sectorSize = 512 // Default sector size
	}

	// Calculate primary GPT size
	// Primary GPT = Protective MBR (1 sector) + GPT Header (1 sector) + Partition Entries (32 sectors)
	primarySectors := int64(gptPrimaryEntriesLbaStart) + int64(gptPartitionEntryCount)
	uncompressedSize := uint64(primarySectors * int64(sectorSize))

	// Extract primary GPT
	rawFile := filepath.Join(outDir, basename+"_gpt.raw")
	err := extractSectors(diskDevPath, rawFile, sectorSize, 0, primarySectors)
	if err != nil {
		return "", 0, fmt.Errorf("%w:\n%w", ErrGptExtractPrimary, err)
	}

	logger.Log.Infof("Extracted primary GPT: %d sectors (%d bytes)",
		primarySectors, uncompressedSize)

	return rawFile, uncompressedSize, nil
}

// extractMbrData extracts just the MBR (first sector) for MBR-partitioned disks
func extractMbrData(diskDevPath string, outDir string, basename string,
	partitionTable *diskutils.PartitionTable,
) (string, uint64, error) {
	sectorSize := partitionTable.SectorSize
	if sectorSize == 0 {
		sectorSize = 512
	}

	// MBR is just the first sector
	rawFile := filepath.Join(outDir, basename+"_mbr.raw")
	err := extractSectors(diskDevPath, rawFile, sectorSize, 0, 1)
	if err != nil {
		return "", 0, fmt.Errorf("failed to extract MBR:\n%w", err)
	}

	return rawFile, uint64(sectorSize), nil
}

// extractSectors extracts a range of sectors from a disk device to a file
func extractSectors(diskDevPath string, outFile string, sectorSize int,
	startSector int64, sectorCount int64,
) error {
	ddArgs := []string{
		fmt.Sprintf("if=%s", diskDevPath),
		fmt.Sprintf("of=%s", outFile),
		fmt.Sprintf("bs=%d", sectorSize),
		fmt.Sprintf("skip=%d", startSector),
		fmt.Sprintf("count=%d", sectorCount),
		"conv=sparse",
	}

	err := shell.ExecuteLive(true, "dd", ddArgs...)
	if err != nil {
		return fmt.Errorf("dd failed (device='%s', start=%d, count=%d):\n%w",
			diskDevPath, startSector, sectorCount, err)
	}

	return nil
}

// compressGptData compresses the raw GPT data file using zstd
func compressGptData(rawFilePath string, outDir string, filename string,
	imageUuid [randomization.UuidSize]byte, compressionLevel int, compressionLong int,
) (string, error) {
	// Define file path for temporary compressed file
	tempCompressedPath := filepath.Join(outDir, filename+"_temp.raw.zst")

	// Compress with zstd
	err := compressWithZstd(rawFilePath, tempCompressedPath, compressionLevel, compressionLong)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrGptExtractCompress, rawFilePath, err)
	}

	// Add skippable frame with image UUID (consistent with partition files)
	finalPath, err := addSkippableFrame(tempCompressedPath, imageUuid, filename, outDir)
	if err != nil {
		os.Remove(tempCompressedPath)
		return "", fmt.Errorf("failed to add skippable frame to GPT file:\n%w", err)
	}

	// Clean up temp file
	err = os.Remove(tempCompressedPath)
	if err != nil {
		return "", fmt.Errorf("%w (file='%s'):\n%w", ErrGptExtractRemoveTempFile, tempCompressedPath, err)
	}

	return finalPath, nil
}

// buildDiskMetadata constructs the Disk struct from extracted GPT data and partition metadata
func buildDiskMetadata(gptData *GptExtractedData, gptImageFile ImageFile, partitionImages map[int]ImageFile) *Disk {
	pt := gptData.PartitionTable

	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = 512
	}

	// Build gptRegions: first the primary-gpt, then each partition in order
	gptRegions := make([]GptDiskRegion, 0, 1+len(pt.Partitions))

	// Add primary-gpt region (always first, startLba is implicitly 0)
	gptRegions = append(gptRegions, GptDiskRegion{
		Image: gptImageFile,
		Type:  RegionTypePrimaryGpt,
		// startLba is omitted for primary-gpt (it's always 0)
	})

	// Add partition regions in order
	for i := range pt.Partitions {
		partNum := i + 1
		partImage, exists := partitionImages[partNum]
		if !exists {
			// Skip partitions without image data (shouldn't happen for valid COSI)
			continue
		}

		region := GptDiskRegion{
			Image:  partImage,
			Type:   RegionTypePartition,
			Number: &partNum,
			// startLba is omitted for partitions (defined in GPT partition entries)
		}
		gptRegions = append(gptRegions, region)
	}

	return &Disk{
		Size:       gptData.DiskSize,
		Type:       DiskTypeGpt,
		LbaSize:    sectorSize,
		GptRegions: gptRegions,
	}
}
