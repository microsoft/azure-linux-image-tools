// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/randomization"
)

var (
	ErrGptExtractReadTable   = NewImageCustomizerError("GptExtract:ReadTable", "failed to read partition table")
	ErrGptExtractNoTable     = NewImageCustomizerError("GptExtract:NoTable", "no partition table found")
	ErrGptExtractPrimary     = NewImageCustomizerError("GptExtract:Primary", "failed to extract primary GPT")
	ErrGptExtractUnsupported = NewImageCustomizerError("GptExtract:Unsupported", "unsupported partition table type")
)

const (
	gptHeaderLba = 1
)

// gptHeader represents the GPT header structure (UEFI Specification 2.10, Table 5-5)
type gptHeader struct {
	Signature                [8]byte
	Revision                 uint32
	HeaderSize               uint32
	HeaderCRC32              uint32
	Reserved                 uint32
	MyLBA                    uint64
	AlternateLBA             uint64
	FirstUsableLBA           uint64
	LastUsableLBA            uint64
	DiskGUID                 [16]byte
	PartitionEntryLBA        uint64
	NumberOfPartitionEntries uint32
	SizeOfPartitionEntry     uint32
	PartitionEntryArrayCRC32 uint32
}

type GptExtractedData struct {
	CompressedFilePath string
	UncompressedSize   uint64
	PartitionTable     *diskutils.PartitionTable
	DiskSize           uint64
}

// extractGptData extracts the GPT partition table data from a disk image.
func extractGptData(diskDevPath string, rawImageFile string, outDir string, basename string,
	imageUuid [randomization.UuidSize]byte, compressionLevel int, compressionLong int,
) (*GptExtractedData, error) {
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

	default:
		return nil, fmt.Errorf("%w (type='%s')", ErrGptExtractUnsupported, partitionTable.Label)
	}

	gptFilename := basename + "_gpt"
	compressedFilePath, err := extractRawZstPartition(rawGptFilePath, imageUuid, gptFilename, outDir,
		compressionLevel, compressionLong)
	if err != nil {
		return nil, err
	}

	logger.Log.Infof("GPT data extracted and compressed: %s (uncompressed size: %d bytes)",
		compressedFilePath, uncompressedSize)

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

func getDiskSize(rawImageFile string) (uint64, error) {
	fileInfo, err := os.Stat(rawImageFile)
	if err != nil {
		return 0, fmt.Errorf("failed to stat disk image file (%s):\n%w", rawImageFile, err)
	}
	return uint64(fileInfo.Size()), nil
}

// extractGptTableData extracts the primary GPT by reading the GPT header to determine
// the actual partition entry array layout.
func extractGptTableData(diskDevPath string, outDir string, basename string,
	partitionTable *diskutils.PartitionTable,
) (string, uint64, error) {
	sectorSize := partitionTable.SectorSize
	if sectorSize == 0 {
		sectorSize = 512
	}

	srcFile, err := os.Open(diskDevPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open disk device (%s):\n%w", diskDevPath, err)
	}
	defer srcFile.Close()

	gptEndBytes, err := readGptEndOffset(srcFile, sectorSize)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read GPT header:\n%w", err)
	}

	// Round up to sector boundary
	sectorsToExtract := gptEndBytes / uint64(sectorSize)
	if gptEndBytes%uint64(sectorSize) != 0 {
		sectorsToExtract += 1
	}

	rawFile := filepath.Join(outDir, basename+"_gpt.raw")
	err = extractSectorsFromFile(srcFile, rawFile, sectorSize, 0, int64(sectorsToExtract))
	if err != nil {
		return "", 0, fmt.Errorf("%w:\n%w", ErrGptExtractPrimary, err)
	}

	logger.Log.Infof("Extracted primary GPT: %d bytes (%d sectors extracted)",
		gptEndBytes, sectorsToExtract)

	return rawFile, gptEndBytes, nil
}

// readGptEndOffset reads the GPT header and calculates the byte offset of the end of the GPT entries.
func readGptEndOffset(file *os.File, sectorSize int) (uint64, error) {
	gptHeaderOffset := int64(gptHeaderLba * sectorSize)
	_, err := file.Seek(gptHeaderOffset, io.SeekStart)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to GPT header:\n%w", err)
	}

	var header gptHeader
	err = binary.Read(file, binary.LittleEndian, &header)
	if err != nil {
		return 0, fmt.Errorf("failed to read GPT header:\n%w", err)
	}

	if header.NumberOfPartitionEntries == 0 || header.SizeOfPartitionEntry == 0 {
		return 0, fmt.Errorf("invalid GPT header: numPartitionEntries=%d, partitionEntrySize=%d",
			header.NumberOfPartitionEntries, header.SizeOfPartitionEntry)
	}

	partitionArraySize := uint64(header.NumberOfPartitionEntries) * uint64(header.SizeOfPartitionEntry)
	partitionArrayStart := header.PartitionEntryLBA * uint64(sectorSize)
	gptEndOffset := partitionArrayStart + partitionArraySize

	logger.Log.Debugf("GPT header parsed: PartitionEntryLBA=%d, NumEntries=%d, EntrySize=%d, EndOffset=%d",
		header.PartitionEntryLBA, header.NumberOfPartitionEntries, header.SizeOfPartitionEntry, gptEndOffset)

	return gptEndOffset, nil
}

func extractSectorsFromFile(srcFile *os.File, outFile string, sectorSize int,
	startSector int64, sectorCount int64,
) error {
	startOffset := startSector * int64(sectorSize)
	_, err := srcFile.Seek(startOffset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to offset %d:\n%w", startOffset, err)
	}

	dstFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create output file (%s):\n%w", outFile, err)
	}
	defer dstFile.Close()

	bytesToCopy := sectorCount * int64(sectorSize)
	_, err = io.Copy(dstFile, io.LimitReader(srcFile, bytesToCopy))
	if err != nil {
		return fmt.Errorf("failed to copy %d bytes:\n%w", bytesToCopy, err)
	}

	err = dstFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close output file (%s):\n%w", outFile, err)
	}

	return nil
}

// buildDiskMetadata constructs the Disk struct from extracted GPT data and partition metadata.
func buildDiskMetadata(gptData *GptExtractedData, gptImageFile ImageFile, partitionImages map[int]ImageFile) (*Disk, error) {
	pt := gptData.PartitionTable

	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = 512
	}

	gptRegions := make([]GptDiskRegion, 0, 1+len(pt.Partitions))

	gptRegions = append(gptRegions, GptDiskRegion{
		Image: gptImageFile,
		Type:  RegionTypePrimaryGpt,
	})

	for _, partition := range pt.Partitions {
		partNum, err := getPartitionNum(partition.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get partition number from path (%s):\n%w", partition.Path, err)
		}

		partImage, exists := partitionImages[partNum]
		if !exists {
			continue
		}

		region := GptDiskRegion{
			Image:  partImage,
			Type:   RegionTypePartition,
			Number: &partNum,
		}
		gptRegions = append(gptRegions, region)
	}

	return &Disk{
		Size:       gptData.DiskSize,
		Type:       DiskTypeGpt,
		LbaSize:    sectorSize,
		GptRegions: gptRegions,
	}, nil
}
