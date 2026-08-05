// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

var (
	ErrResizePartMultiSpecified = NewImageCustomizerError("Partitions:ResizePartMultiSpecified", "partition specified multiple times for resize")
	ErrResizeDisk               = NewImageCustomizerError("Partitions:ResizeDiskFile", "failed to resize disk")
	ErrResizeLastPartition      = NewImageCustomizerError("Partitions:ResizeLastPartition", "failed to resize last partition in disk")
)

func resizeDiskAndPartitions(ctx context.Context, buildImageFile string, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	if len(resizeConfig.Partitions) <= 0 {
		// Nothing to do.
		return nil
	}

	logger.Log.Infof("Resize disk and partitions")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "resize_disk")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer loopback.Close()

	err = resizeDiskAndPartitionsHelper(loopback, buildDir, resizeConfig)
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func resizeDiskAndPartitionsHelper(loopback *safeloopback.Loopback, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	partitionTablePtr, err := diskutils.ReadDiskPartitionTable(loopback.DevicePath())
	if err != nil {
		return err
	}

	if partitionTablePtr == nil {
		return fmt.Errorf("unexpected empty disk")
	}

	partitionTable := *partitionTablePtr

	lastPartIndex, _, err := findLastPartition(partitionTable)
	if err != nil {
		return err
	}

	newSizes := make(map[int]uint64)

	for _, resizePartConfig := range resizeConfig.Partitions {
		partIndex, err := resolvePartitionForResizeOp(resizePartConfig, partitionTable, lastPartIndex)
		if err != nil {
			return err
		}

		_, alreadySeen := newSizes[partIndex]
		if alreadySeen {
			return fmt.Errorf("%w (ref=%s')", ErrResizePartMultiSpecified, resizePartConfig.Ref)
		}

		//
		// TODO: Handle if partition uses inline verity.
		//

		update, newPartSize, err := calcNewPartitionSize(resizePartConfig, partitionTable.Partitions[partIndex],
			uint64(partitionTable.SectorSize), buildDir)
		if err != nil {
			return err
		}

		if !update {
			// Nothing to do.
			return nil
		}

		newSizes[partIndex] = newPartSize
	}

	if len(newSizes) == 1 {
		newPartSize, isLastPart := newSizes[lastPartIndex]
		if isLastPart {
			err := resizeDiskLastPartition(loopback, newPartSize, lastPartIndex, partitionTable)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrResizeLastPartition, err)
			}

			return nil
		}
	}

	return fmt.Errorf("cannot yet resize partitions other than the last partition")
}

func resizeDiskLastPartition(loopback *safeloopback.Loopback, newPartSize uint64, lastPartIndex int,
	partitionTable diskutils.PartitionTable,
) error {
	lastPartInfo := partitionTable.Partitions[lastPartIndex]
	newPartSizeInSectors := convertBytesToSectors(newPartSize, uint64(partitionTable.SectorSize))
	newPartSize = newPartSizeInSectors * uint64(partitionTable.SectorSize)

	newDiskSize := (uint64(lastPartInfo.Start)+newPartSizeInSectors)*uint64(partitionTable.SectorSize) +
		imagecustomizerapi.GptHeaderSize

	logger.Log.Debugf("Resizing disk (newSize='%s')", imagecustomizerapi.DiskSize(newDiskSize).HumanReadable())

	// Resize the disk file.
	// Note: We don't need to resize the GPT layout here, since it will be fixed when diskutils.ResizePartition is
	// called below.
	err := loopback.Resize(int64(newDiskSize))
	if err != nil {
		return fmt.Errorf("%w (size='%s'):\n%w", ErrResizeDisk,
			imagecustomizerapi.DiskSize(newDiskSize).HumanReadable(), err)
	}

	// Resize the partition.
	err = diskutils.ResizePartition(lastPartInfo.Path, loopback.DevicePath(), newPartSizeInSectors)
	if err != nil {
		return err
	}

	return nil
}

func resolvePartitionForResizeOp(resizePartConfig imagecustomizerapi.ResizePartition,
	partitionTable diskutils.PartitionTable, lastPartIndex int,
) (int, error) {
	switch {
	case resizePartConfig.Ref == imagecustomizerapi.ResizePartitionRefLast:
		return lastPartIndex, nil

	default:
		// This should be prevented by ResizePartition.IsValid().
		err := fmt.Errorf("no reference provided for partition resize")
		return 0, err
	}
}

func findLastPartition(partitionTable diskutils.PartitionTable) (int, diskutils.PartitionTablePartition, error) {
	if len(partitionTable.Partitions) <= 0 {
		err := fmt.Errorf("disk has no partitions")
		return 0, diskutils.PartitionTablePartition{}, err
	}

	lastPartitionIndex := 0
	lastPartition := partitionTable.Partitions[0]
	for i := 1; i < len(partitionTable.Partitions); i++ {
		if partitionTable.Partitions[i].Start > lastPartition.Start {
			lastPartitionIndex = i
			lastPartition = partitionTable.Partitions[i]
		}
	}

	return lastPartitionIndex, lastPartition, nil
}

func calcNewPartitionSize(resizePartConfig imagecustomizerapi.ResizePartition,
	partInfo diskutils.PartitionTablePartition, sectorSize uint64, buildDir string,
) (bool, uint64, error) {
	switch {
	case resizePartConfig.FreeSpace != nil:
		return calcNewPartitionSizeWithFreeSpace(uint64(*resizePartConfig.FreeSpace), partInfo, sectorSize, buildDir)

	default:
		// This should be prevented by ResizePartition.IsValid().
		return false, 0, fmt.Errorf("no size specified for partition resize operation")
	}
}

// Calculate the new size of a partition so that it has the specified amount of free space (or more).
func calcNewPartitionSizeWithFreeSpace(requiredFreeSpace uint64, partInfo diskutils.PartitionTablePartition,
	sectorSize uint64, buildDir string,
) (bool, uint64, error) {
	partSize := uint64(partInfo.Size) * sectorSize

	_, freeSpace, err := getPartitionsFilesystemSize(buildDir, partInfo)
	if err != nil {
		return false, 0, err
	}

	if freeSpace >= uint64(requiredFreeSpace) {
		// Nothing to do.
		return false, partSize, nil
	}

	missingFreeSpace := requiredFreeSpace - freeSpace
	newPartSize := partSize + missingFreeSpace
	newPartSize = diskutils.RoundUp(newPartSize, imagecustomizerapi.DefaultPartitionAlignment)

	logger.Log.Debugf("Partition new size (path='%s', freeSpace='%s', requiredFreeSpace='%s', newSize='%s')",
		partInfo.Path,
		imagecustomizerapi.DiskSize(freeSpace).HumanReadable(),
		imagecustomizerapi.DiskSize(requiredFreeSpace).HumanReadable(),
		imagecustomizerapi.DiskSize(newPartSize).HumanReadable())

	return true, newPartSize, nil
}

// Get the size of a partition's filesystem and its free space.
func getPartitionsFilesystemSize(buildDir string, partInfo diskutils.PartitionTablePartition) (uint64, uint64, error) {
	mountDir := filepath.Join(buildDir, "tmp-last-partition")

	mount, err := safemount.NewMount(partInfo.Path, mountDir, partInfo.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return 0, 0, err
	}
	defer mount.Close()

	var stat unix.Statfs_t
	err = unix.Statfs(mountDir, &stat)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read partition stats:\n%w", err)
	}

	totalBytes := uint64(stat.Frsize) * stat.Blocks
	freeBytes := uint64(stat.Frsize) * stat.Bfree

	err = mount.CleanClose()
	if err != nil {
		return 0, 0, err
	}

	return totalBytes, freeBytes, nil
}
