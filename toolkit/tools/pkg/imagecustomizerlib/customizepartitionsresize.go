// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/mathutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

var (
	ErrResizePartMultiSpecified              = NewImageCustomizerError("Partitions:ResizePartMultiSpecified", "partition specified multiple times for resize")
	ErrResizeLastPartition                   = NewImageCustomizerError("Partitions:ResizeLastPartition", "failed to resize last partition in disk")
	ErrDiskSizeNotMultipleOfSectorSize       = NewImageCustomizerError("Partitions:DiskSizeNotMultipleOfSectorSize", "disk file size is not a multiple of the sector size")
	ErrDiskFileResizeFailed                  = NewImageCustomizerError("Partitions:DiskFileResizeFailed", "failed to resize the disk file")
	ErrPartitionResizeFailed                 = NewImageCustomizerError("Partitions:PartitionResizeFailed", "failed to resize partition")
	ErrAdvancedPartitionResizeNotImplemented = NewImageCustomizerError("Partitions:AdvancedPartitionResizeNotImplemented", "cannot yet resize partitions other than the last partition")
	ErrFileSystemResizeNotSupported          = NewImageCustomizerError("Partitions:FileSystemResizeNotSupported", "resizing filesystem is not supported")
	ErrResizeShrinkPartitionsNotSupported    = NewImageCustomizerError("Partitions:ResizeShrinkPartitionsNotSupported", "shrinking partitions during disk resize is not supported")
	ErrResizeShrinkDiskNotSupported          = NewImageCustomizerError("Partitions:ResizeShrinkDiskNotSupported", "shrinking disk during disk resize is not supported")
	ErrReadFileSystemSizeFailed              = NewImageCustomizerError("Partitions:ReadFileSystemSizeFailed", "failed to read filesystem size")
)

func resizeDiskAndPartitions(ctx context.Context, buildImageFile string, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	logger.Log.Infof("Resize disk and partitions")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "resize_disk")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer loopback.Close()

	imageFile, err := os.OpenFile(buildImageFile, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}

	err = resizeDiskAndPartitionsHelper(loopback, imageFile, buildDir, resizeConfig)
	if err != nil {
		return err
	}

	err = imageFile.Close()
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func resizeDiskAndPartitionsHelper(loopback *safeloopback.Loopback, imageFile *os.File,
	buildDir string, resizeConfig imagecustomizerapi.ResizeDisk,
) error {
	imageFileInfo, err := imageFile.Stat()
	if err != nil {
		return err
	}

	diskSize := imageFileInfo.Size()

	partitionTablePtr, err := diskutils.ReadDiskPartitionTable(loopback.DevicePath())
	if err != nil {
		return fmt.Errorf("%w (device='%s'):\n%w", ErrGptExtractReadTable, loopback.DevicePath(), err)
	}

	if partitionTablePtr == nil {
		return fmt.Errorf("%w (file='%s')", ErrGptExtractNoTable, loopback.DiskFilePath())
	}

	partitionTable := *partitionTablePtr

	gptHeaderSize, err := readGptHeaderSize(loopback.DevicePath(), partitionTable.SectorSize)
	if err != nil {
		return err
	}

	gptFooterSize := gptHeaderSize - uint64(partitionTable.SectorSize)

	diskSizeInSectors := diskSize / int64(partitionTable.SectorSize)
	if diskSize%int64(partitionTable.SectorSize) != 0 {
		return fmt.Errorf("%w (file='%s', diskSize=%d, sectorSize=%d)", ErrDiskSizeNotMultipleOfSectorSize,
			loopback.DiskFilePath(), diskSize, partitionTable.SectorSize)
	}

	lastPartIndex, _, err := findLastPartition(partitionTable)
	if err != nil {
		return err
	}

	var newPartSizes map[int]uint64

	switch {
	case len(resizeConfig.Partitions) > 0:
		newPartSizes, err = resolveResizeWithPartitions(resizeConfig, partitionTable, lastPartIndex, buildDir)
		if err != nil {
			return err
		}

	case resizeConfig.DiskSize != nil:
		newPartSizes, err = resolveResizeWithDiskSize(uint64(*resizeConfig.DiskSize), partitionTable, lastPartIndex,
			diskSize, gptFooterSize)
		if err != nil {
			return err
		}

	default:
		// Not possible, since it should be pre-checked by ResizeConfig.IsValid().
		return fmt.Errorf("empty diskResize config")
	}

	if len(newPartSizes) == 1 {
		newPartSize, isLastPart := newPartSizes[lastPartIndex]
		if isLastPart {
			err := resizeDiskLastPartition(loopback, imageFile, diskSizeInSectors, newPartSize, lastPartIndex,
				partitionTable, gptFooterSize, buildDir)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrResizeLastPartition, err)
			}

			return nil
		}
	}

	return ErrAdvancedPartitionResizeNotImplemented
}

func resolveResizeWithPartitions(resizeConfig imagecustomizerapi.ResizeDisk, partitionTable diskutils.PartitionTable,
	lastPartIndex int, buildDir string,
) (map[int]uint64, error) {
	newPartSizes := make(map[int]uint64)

	for _, resizePartConfig := range resizeConfig.Partitions {
		partIndex, err := resolvePartitionForResizeOp(resizePartConfig, partitionTable, lastPartIndex)
		if err != nil {
			return nil, err
		}

		_, alreadySeen := newPartSizes[partIndex]
		if alreadySeen {
			return nil, fmt.Errorf("%w (ref=%s')", ErrResizePartMultiSpecified, resizePartConfig.Ref)
		}

		update, newPartSize, err := calcNewPartitionSize(resizePartConfig, partitionTable.Partitions[partIndex],
			partitionTable.SectorSize, buildDir)
		if err != nil {
			return nil, err
		}

		if !update {
			// Nothing to do.
			continue
		}

		newPartSizes[partIndex] = newPartSize
	}

	return newPartSizes, nil
}

func resolveResizeWithDiskSize(newDiskSize uint64, partitionTable diskutils.PartitionTable,
	lastPartIndex int, diskSize int64, gptFooterSize uint64,
) (map[int]uint64, error) {
	newPartSizes := make(map[int]uint64)

	if newDiskSize < uint64(diskSize) {
		return nil, fmt.Errorf("%w (currSize=%s, newSize=%s)", ErrResizeShrinkDiskNotSupported,
			diskutils.HumanReadable(diskSize), diskutils.HumanReadable(newDiskSize))
	}
	if newDiskSize == uint64(diskSize) {
		// Nothing to do.
		return newPartSizes, nil
	}

	lastPartInfo := partitionTable.Partitions[lastPartIndex]
	lastPartSize := uint64(lastPartInfo.Size) * uint64(partitionTable.SectorSize)
	lastPartStart := uint64(lastPartInfo.Start) * uint64(partitionTable.SectorSize)

	newLastPartSize := newDiskSize - uint64(gptFooterSize) - lastPartStart
	newLastPartSize = mathutils.RoundDown(newLastPartSize, imagecustomizerapi.DefaultPartitionAlignment)

	if newLastPartSize < lastPartSize {
		return nil, fmt.Errorf("%w (currSize=%s, newSize=%s)", ErrResizeShrinkPartitionsNotSupported,
			diskutils.HumanReadable(lastPartSize), diskutils.HumanReadable(newLastPartSize))
	}

	if newLastPartSize == lastPartSize {
		// Nothing to do.
		return newPartSizes, nil
	}

	newPartSizes[lastPartIndex] = newLastPartSize
	return newPartSizes, nil
}

func resizeDiskLastPartition(loopback *safeloopback.Loopback, imageFile *os.File, diskSizeInSectors int64,
	newLastPartSize uint64, lastPartIndex int, partitionTable diskutils.PartitionTable, gptFooterSize uint64,
	buildDir string,
) error {
	lastPartInfo := partitionTable.Partitions[lastPartIndex]

	newLastPartSizeInSectors := newLastPartSize / uint64(partitionTable.SectorSize)
	if newLastPartSize%uint64(partitionTable.SectorSize) != 0 {
		// Shouldn't happen, since the size should be rounded up to DefaultPartitionAlignment, which should be a
		// multiple of the sector size.
		return fmt.Errorf("new partition size is not a multiple of the sector size (partSize=%d, sectorSize=%d)",
			newLastPartSize, partitionTable.SectorSize)
	}

	newLastPartEnd := (uint64(lastPartInfo.Start) + newLastPartSizeInSectors) * uint64(partitionTable.SectorSize)

	newDiskSize := newLastPartEnd + gptFooterSize
	newDiskSize = mathutils.RoundUp(newDiskSize, imagecustomizerapi.DefaultPartitionAlignment)

	logger.Log.Infof("Last partition size: %s -> %s",
		diskutils.HumanReadable(uint64(lastPartInfo.Size)*uint64(partitionTable.SectorSize)),
		diskutils.HumanReadable(newLastPartSize))
	logger.Log.Infof("Disk size: %s -> %s",
		diskutils.HumanReadable(uint64(diskSizeInSectors)*uint64(partitionTable.SectorSize)),
		diskutils.HumanReadable(newDiskSize))

	// Resize the disk.
	err := imageFile.Truncate(int64(newDiskSize))
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrDiskFileResizeFailed, err)
	}

	err = loopback.RefreshDiskSize()
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrDiskFileResizeFailed, err)
	}

	// Resize the last partition.
	err = growPartitionSize(loopback.DevicePath(), lastPartInfo.Path, newLastPartSizeInSectors,
		lastPartInfo.FileSystemType, buildDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrPartitionResizeFailed, err)
	}

	return nil
}

func growPartitionSize(devicePath string, partPath string, newPartSizeInSectors uint64, partFileSystemType string,
	buildDir string,
) error {
	// Resize the partition.
	// Note: sfdisk will also fix the GPT footer's location at the same time.
	err := diskutils.ResizePartition(partPath, devicePath, newPartSizeInSectors)
	if err != nil {
		return err
	}

	// Resize the filesystem.
	err = growFileSystem(devicePath, partPath, partFileSystemType, buildDir)
	if err != nil {
		return err
	}

	return nil
}

func growFileSystem(diskDevicePath string, partitionPath string, fileSystemType string, buildDir string) error {
	switch fileSystemType {
	case "ext4":
		return growExt4(diskDevicePath, partitionPath)

	case "xfs":
		return growXfs(diskDevicePath, partitionPath, buildDir)

	case "btrfs":
		return growBtrfs(diskDevicePath, partitionPath, buildDir)

	default:
		return fmt.Errorf("%w (type='%s')", ErrFileSystemResizeNotSupported, fileSystemType)
	}
}

func growExt4(diskDevicePath string, partitionPath string) error {
	// resize2fs requires e2fsck to be called before resize2fs is called.
	err := shell.NewExecBuilder("e2fsck", "-fy", partitionPath).
		Execute()
	if err != nil {
		return fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemE2fsckResize, partitionPath, err)
	}

	err = shell.NewExecBuilder("flock", "--timeout", "5", diskDevicePath, "resize2fs", partitionPath).
		Execute()
	if err != nil {
		return fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemResize2fs, partitionPath, err)
	}

	return nil
}

func growXfs(diskDevicePath string, partitionPath string, buildDir string) error {
	mountPath := filepath.Join(buildDir, "resizefs")

	// xfs_growfs requires the filesystem to be mounted.
	mount, err := safemount.NewMount(partitionPath, mountPath, "xfs", 0, "", true /*makeAndDeleteDir*/)
	if err != nil {
		return err
	}
	defer mount.Close()

	err = shell.NewExecBuilder("flock", "--timeout", "5", diskDevicePath, "xfs_growfs", mount.Target()).
		Execute()
	if err != nil {
		return fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemResize2fs, partitionPath, err)
	}

	err = mount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func growBtrfs(diskDevicePath string, partitionPath string, buildDir string) error {
	mountPath := filepath.Join(buildDir, "resizefs")

	// 'btrfs filesystem resize' requires the filesystem to be mounted.
	// Use "subvolid=5" to mount the top-level subvolume instead of the default subvolume.
	mount, err := safemount.NewMount(partitionPath, mountPath, "btrfs", 0, "subvolid=5", true /*makeAndDeleteDir*/)
	if err != nil {
		return err
	}
	defer mount.Close()

	err = shell.NewExecBuilder("flock", "--timeout", "5", diskDevicePath,
		"btrfs", "filesystem", "resize", "max", mount.Target()).
		Execute()
	if err != nil {
		return fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemResize2fs, partitionPath, err)
	}

	err = mount.CleanClose()
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
	partInfo diskutils.PartitionTablePartition, sectorSize int, buildDir string,
) (bool, uint64, error) {
	partSize := uint64(partInfo.Size) * uint64(sectorSize)

	switch {
	case resizePartConfig.FreeSpace != nil:
		return calcNewPartitionSizeWithFreeSpace(uint64(*resizePartConfig.FreeSpace), partSize, partInfo.Path,
			partInfo.FileSystemType, buildDir)

	case resizePartConfig.Size != nil:
		return calcNewPartitionSizeWithSize(uint64(*resizePartConfig.Size), partSize)

	default:
		// This should be prevented by ResizePartition.IsValid().
		return false, 0, fmt.Errorf("no size specified for partition resize operation")
	}
}

// Calculate the new size of a partition so that it has the specified amount of free space (or more).
func calcNewPartitionSizeWithFreeSpace(requiredFreeSpace uint64, partSize uint64, partPath string, partFsType string,
	buildDir string,
) (bool, uint64, error) {
	_, freeSpace, err := getPartitionsFilesystemSize(buildDir, partPath, partFsType)
	if err != nil {
		return false, 0, fmt.Errorf("%w:\n%w", ErrReadFileSystemSizeFailed, err)
	}

	logger.Log.Debugf("Partition free space resize: free=%s, freeRequired=%s",
		diskutils.HumanReadable(freeSpace),
		diskutils.HumanReadable(requiredFreeSpace))

	if freeSpace >= uint64(requiredFreeSpace) {
		// Nothing to do.
		return false, partSize, nil
	}

	missingFreeSpace := requiredFreeSpace - freeSpace
	newPartitionSize := partSize + missingFreeSpace
	newPartitionSize = mathutils.RoundUp(newPartitionSize, imagecustomizerapi.DefaultPartitionAlignment)

	return true, newPartitionSize, nil
}

// Get the size of a partition's filesystem and its free space.
func getPartitionsFilesystemSize(buildDir string, partPath string, partFsType string) (uint64, uint64, error) {
	mountDir := filepath.Join(buildDir, "tmp-last-partition")

	mount, err := safemount.NewMount(partPath, mountDir, partFsType, unix.MS_RDONLY, "", true)
	if err != nil {
		return 0, 0, err
	}
	defer mount.Close()

	var stat unix.Statfs_t
	err = unix.Statfs(mountDir, &stat)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read filesystem stats:\n%w", err)
	}

	totalBytes := uint64(stat.Frsize) * stat.Blocks
	freeBytes := uint64(stat.Frsize) * stat.Bfree

	err = mount.CleanClose()
	if err != nil {
		return 0, 0, err
	}

	return totalBytes, freeBytes, nil
}

// Calculate the new size of a partition so that it has the specified amount of free space (or more).
func calcNewPartitionSizeWithSize(newSize uint64, partSize uint64) (bool, uint64, error) {
	if newSize < partSize {
		return false, 0, fmt.Errorf("%w (currSize=%s, newSize=%s)", ErrResizeShrinkPartitionsNotSupported,
			diskutils.HumanReadable(partSize), diskutils.HumanReadable(newSize))
	}

	if newSize == partSize {
		return false, newSize, nil
	}

	return true, newSize, nil
}
