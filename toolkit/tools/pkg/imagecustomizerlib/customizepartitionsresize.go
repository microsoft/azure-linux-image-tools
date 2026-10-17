// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/mathutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

var (
	ErrResizePartMultiSpecified               = NewImageCustomizerError("Partitions:ResizePartMultiSpecified", "partition specified multiple times for resize")
	ErrResizeLastPartition                    = NewImageCustomizerError("Partitions:ResizeLastPartition", "failed to resize last partition in disk")
	ErrDiskSizeNotMultipleOfSectorSize        = NewImageCustomizerError("Partitions:DiskSizeNotMultipleOfSectorSize", "disk file size is not a multiple of the sector size")
	ErrDiskFileResizeFailed                   = NewImageCustomizerError("Partitions:DiskFileResizeFailed", "failed to resize the disk file")
	ErrPartitionResizeFailed                  = NewImageCustomizerError("Partitions:PartitionResizeFailed", "failed to resize partition")
	ErrAdvancedPartitionResizeNotImplemented  = NewImageCustomizerError("Partitions:AdvancedPartitionResizeNotImplemented", "cannot yet resize partitions other than the last partition")
	ErrFileSystemResizeNotSupported           = NewImageCustomizerError("Partitions:FileSystemResizeNotSupported", "resizing filesystem is not supported")
	ErrResizeShrinkPartitionsNotSupported     = NewImageCustomizerError("Partitions:ResizeShrinkPartitionsNotSupported", "shrinking partitions during disk resize is not supported")
	ErrResizeShrinkDiskNotSupported           = NewImageCustomizerError("Partitions:ResizeShrinkDiskNotSupported", "shrinking disk during disk resize is not supported")
	ErrReadFileSystemSizeFailed               = NewImageCustomizerError("Partitions:ReadFileSystemSizeFailed", "failed to read filesystem size")
	ErrDiskResizeResolveFailed                = NewImageCustomizerError("Partitions:ResizeResolveFailed", "failed to resolve disk resize operation")
	ErrDiskResizeFindPartitionFailed          = NewImageCustomizerError("Partitions:ResizeFindPartitionFailed", "failed to find partition")
	ErrDiskResizeFindTooManyPartitions        = NewImageCustomizerError("Partitions:ResizeFindTooManyPartitions", "partition reference matches more than 1 partition")
	ErrDiskResizeFailedToCreatePartitionTable = NewImageCustomizerError("Partitions:DiskResizeFailedToCreatePartitionTable", "failed to create partition table")
	ErrDiskResizePartitionCopyFailed          = NewImageCustomizerError("Partitions:DiskResizePartitionCopyFailed", "failed to copy partition to the new disk")
	ErrDiskResizeCannotMoveBiosBootPartition  = NewImageCustomizerError("Partitions:DiskResizeCannotMoveBiosBootPartition", "cannot move bios boot partition")
)

func resizeDiskAndPartitions(ctx context.Context, buildImageFile string, buildDir string,
	resizeConfig imagecustomizerapi.ResizeDisk,
) (string, error) {
	logger.Log.Infof("Resize disk and partitions")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "resize_disk")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return "", err
	}
	defer loopback.Close()

	imageFile, err := os.OpenFile(buildImageFile, os.O_RDWR, os.ModePerm)
	if err != nil {
		return "", err
	}

	newImageFilePath, err := resizeDiskAndPartitionsHelper(loopback, imageFile, buildDir, resizeConfig)
	if err != nil {
		return "", err
	}

	if newImageFilePath == "" {
		newImageFilePath = buildImageFile
	}

	err = imageFile.Close()
	if err != nil {
		return "", err
	}

	err = loopback.CleanClose()
	if err != nil {
		return "", err
	}

	return newImageFilePath, nil
}

func resizeDiskAndPartitionsHelper(loopback *safeloopback.Loopback, imageFile *os.File,
	buildDir string, resizeConfig imagecustomizerapi.ResizeDisk,
) (string, error) {
	imageFileInfo, err := imageFile.Stat()
	if err != nil {
		return "", err
	}

	diskSize := imageFileInfo.Size()

	partitionTablePtr, err := diskutils.ReadDiskPartitionTable(loopback.DevicePath())
	if err != nil {
		return "", fmt.Errorf("%w (device='%s'):\n%w", ErrGptExtractReadTable, loopback.DevicePath(), err)
	}

	if partitionTablePtr == nil {
		return "", fmt.Errorf("%w (file='%s')", ErrGptExtractNoTable, loopback.DiskFilePath())
	}

	partitionTable := *partitionTablePtr

	gptHeaderSize, err := readGptHeaderSize(loopback.DevicePath(), partitionTable.SectorSize)
	if err != nil {
		return "", err
	}

	// Subtract the MBR header sector.
	gptFooterSize := gptHeaderSize - uint64(partitionTable.SectorSize)

	if diskSize%int64(partitionTable.SectorSize) != 0 {
		return "", fmt.Errorf("%w (file='%s', diskSize=%d, sectorSize=%d)", ErrDiskSizeNotMultipleOfSectorSize,
			loopback.DiskFilePath(), diskSize, partitionTable.SectorSize)
	}

	lastPartIndex, _, err := findLastPartition(partitionTable)
	if err != nil {
		return "", err
	}

	// Calculate the resize operation.
	var newPartSizes map[int]uint64

	switch {
	case len(resizeConfig.Partitions) > 0:
		newPartSizes, err = resolveResizeWithPartitions(resizeConfig, partitionTable, lastPartIndex, buildDir)
		if err != nil {
			return "", fmt.Errorf("%w:\n%w", ErrDiskResizeResolveFailed, err)
		}

	case resizeConfig.DiskSize != nil:
		newPartSizes, err = resolveResizeWithDiskSize(uint64(*resizeConfig.DiskSize), partitionTable, lastPartIndex,
			diskSize, gptFooterSize)
		if err != nil {
			return "", fmt.Errorf("%w:\n%w", ErrDiskResizeResolveFailed, err)
		}

	default:
		// Not possible, since it should be pre-checked by ResizeConfig.IsValid().
		return "", fmt.Errorf("empty diskResize config")
	}

	if len(newPartSizes) == 0 {
		// Nothing to do.
		return "", nil
	}

	// Do the resize operation.
	if len(newPartSizes) == 1 {
		newPartSize, isLastPart := newPartSizes[lastPartIndex]
		if isLastPart {
			// Expand the disk and the last partition.
			err := resizeDiskLastPartition(loopback, imageFile, diskSize, newPartSize, lastPartIndex,
				partitionTable, gptFooterSize, buildDir)
			if err != nil {
				return "", fmt.Errorf("%w:\n%w", ErrResizeLastPartition, err)
			}

			return "", nil
		}
	}

	newImageFilePath, err := resizeDiskAdvanced(loopback, partitionTable, diskSize, newPartSizes, gptHeaderSize,
		gptFooterSize, buildDir)
	if err != nil {
		return "", err
	}

	return newImageFilePath, nil
}

// Calculate the disk resize that expands expands the last partition and the disk size to contain the last partition.
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

// Calculate the disk resize that expands the size of the disk and the last partition to fill the disk size.
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

// Perform a disk resize the expands the size of the disk and the last partition.
// This is a special case since it can be performed without moving data.
func resizeDiskLastPartition(loopback *safeloopback.Loopback, imageFile *os.File, diskSize int64,
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
		diskutils.HumanReadable(diskSize),
		diskutils.HumanReadable(newDiskSize))

	// Resize the disk.
	err := imageFile.Truncate(int64(newDiskSize))
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrDiskFileResizeFailed, err)
	}

	// Update the size of the loopback block device to match the new file size.
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

// Resize the disk and partitions, where any partition can be resized so partitions may need to be moved around.
func resizeDiskAdvanced(loopback *safeloopback.Loopback, partitionTable diskutils.PartitionTable, diskSize int64,
	newPartSizes map[int]uint64, gptHeaderSize uint64, gptFooterSize uint64, buildDir string,
) (string, error) {
	newImageFilePath := filepath.Join(buildDir, PartitionCustomizedImageName)

	newPartitionTable, newDiskSize, err := resolveResizeDiskAdvanced(partitionTable, newPartSizes, gptHeaderSize,
		gptFooterSize)
	if err != nil {
		return "", err
	}

	sectorSize := int64(partitionTable.SectorSize)

	sizeCompareStr := func(oldSize, newSize int64) string {
		if oldSize == newSize {
			return diskutils.HumanReadable(oldSize)
		} else {
			return fmt.Sprintf("%s -> %s", diskutils.HumanReadable(oldSize), diskutils.HumanReadable(newSize))
		}
	}

	logger.Log.Infof("Disk size: %s", sizeCompareStr(diskSize, int64(newDiskSize)))
	for i := range partitionTable.Partitions {
		oldPart := partitionTable.Partitions[i]
		newPart := newPartitionTable.Partitions[i]

		logger.Log.Infof("Partition (%s): Start: %s, Size: %s", newPart.Path,
			sizeCompareStr(oldPart.Start*sectorSize, newPart.Start*sectorSize),
			sizeCompareStr(oldPart.Size*sectorSize, newPart.Size*sectorSize))
	}

	err = createNewResizedDisk(loopback.DevicePath(), newImageFilePath, newDiskSize, gptHeaderSize, gptFooterSize,
		newPartitionTable)
	if err != nil {
		return "", err
	}

	newDiskLoopback, err := safeloopback.NewLoopback(newImageFilePath)
	if err != nil {
		return "", err
	}
	defer newDiskLoopback.Close()

	err = copyAndGrowPartitions(loopback.DevicePath(), newDiskLoopback.DevicePath(), partitionTable, newPartitionTable,
		buildDir)
	if err != nil {
		return "", err
	}

	err = newDiskLoopback.CleanClose()
	if err != nil {
		return "", err
	}

	return newImageFilePath, nil
}

// Copies the partitions from the old disk to the new disk and then expands the filesystems to fill the new partition
// sizes.
func copyAndGrowPartitions(oldDiskDevice string, newDiskDevice string, oldPartitionTable diskutils.PartitionTable,
	newPartitionTable diskutils.PartitionTable, buildDir string,
) error {
	for i := range oldPartitionTable.Partitions {
		oldPart := oldPartitionTable.Partitions[i]
		newPart := newPartitionTable.Partitions[i]

		if oldPart.Size > newPart.Size {
			// Shouldn't happen since this is checked while calculating the new layout.
			return fmt.Errorf("old partition size is larger than the new partition size")
		}

		partNum, err := diskutils.GetPartitionNum(oldPart.Path)
		if err != nil {
			return err
		}

		newPartPath := fmt.Sprintf("%sp%d", newDiskDevice, partNum)

		err = copyPartitionFile(newDiskDevice, oldPart.Path, newPartPath)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrDiskResizePartitionCopyFailed, err)
		}

		if oldPart.Size != newPart.Size {
			// Resize the filesystem to match the new filesystem size.
			err = growFileSystem(newDiskDevice, newPart.Path, newPart.FileSystemType, buildDir)
			if err != nil {
				return fmt.Errorf("%w:\n%w", ErrPartitionResizeFailed, err)
			}
		}
	}

	// Wait for changes to populate in kernel.
	err := diskutils.WaitForDiskDevice(newDiskDevice)
	if err != nil {
		return err
	}

	return nil
}

func copyPartitionFile(diskDevice string, oldPartPath string, newPartPath string) error {
	diskDeviceFile, err := os.Open(diskDevice)
	if err != nil {
		return err
	}
	defer diskDeviceFile.Close()

	_, err = file.LockExclusive(context.Background(), diskDeviceFile, 5*time.Second)
	if err != nil {
		return err
	}

	oldPartFile, err := os.Open(oldPartPath)
	if err != nil {
		return err
	}
	defer oldPartFile.Close()

	newPartFile, err := os.OpenFile(newPartPath, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer newPartFile.Close()

	// Note: io.Copy uses the sendfile syscall where appropriate.
	_, err = io.Copy(newPartFile, oldPartFile)
	if err != nil {
		return err
	}

	// Ensure all the data is successfully written.
	err = newPartFile.Close()
	if err != nil {
		return err
	}

	return nil
}

// Calculate the new partition layout.
func resolveResizeDiskAdvanced(partitionTable diskutils.PartitionTable, newPartSizes map[int]uint64,
	gptHeaderSize uint64, gptFooterSize uint64,
) (diskutils.PartitionTable, uint64, error) {
	alignmentSize := int64(imagecustomizerapi.DefaultPartitionAlignment)
	sectorSize := int64(partitionTable.SectorSize)
	alignmentSectors := alignmentSize / sectorSize
	if alignmentSize%sectorSize != 0 {
		// Shouldn't happen since alignment size is 1 MiB and sector size is either 512 or 4 KiB.
		err := fmt.Errorf("default partition alignment size is not a multiple of the sector size (alignment=%d, sector=%d)",
			alignmentSize, sectorSize)
		return diskutils.PartitionTable{}, 0, err
	}

	indexes := []int(nil)
	for i := range partitionTable.Partitions {
		indexes = append(indexes, i)
	}

	// Some distros (e.g. Ubuntu) have a physical disk order that is different from the table order.
	sortedPartitionIndexes := slices.SortedFunc(slices.Values(indexes),
		func(a, b int) int {
			return cmp.Compare(partitionTable.Partitions[a].Start, partitionTable.Partitions[b].Start)
		})

	// GPT header.
	if partitionTable.FirstLba < int64(gptHeaderSize)/sectorSize {
		err := fmt.Errorf("invalid disk: FirstUsableLba smaller than GPT header size")
		return diskutils.PartitionTable{}, 0, err
	}

	nextSector := partitionTable.FirstLba

	// Assign partition sectors.
	newPartitions := make([]diskutils.PartitionTablePartition, len(partitionTable.Partitions))
	for _, index := range sortedPartitionIndexes {
		partition := partitionTable.Partitions[index]
		newPartition := partition

		if newSize, hasNewSize := newPartSizes[index]; hasNewSize {
			newPartSizeInSectors := newSize / uint64(sectorSize)
			if newSize%uint64(sectorSize) != 0 {
				// Shouldn't happen, since the size should be rounded up to DefaultPartitionAlignment, which should be a
				// multiple of the sector size.
				err := fmt.Errorf("new partition size is not a multiple of the sector size (partSize=%d, sectorSize=%d)",
					newSize, sectorSize)
				return diskutils.PartitionTable{}, 0, err
			}

			// Note: This will fill up any empty space before and after the partition before expanding into existing
			// partitions' territory.
			newPartition.Start = mathutils.RoundUp(nextSector, alignmentSectors)
			newPartition.Size = int64(newPartSizeInSectors)
		} else if partition.Start < nextSector {
			// We can't give the partition its original location.
			// So, we need to move it.
			if partition.PartTypeUuid == imagecustomizerapi.PartitionTypeToUuid[imagecustomizerapi.PartitionTypeBiosGrub] {
				// This is a BIOS boot partition. It's location is hardcoded inside the MBR boot code.
				// So, it can't be moved.
				err := fmt.Errorf("%w (oldStart=%s, newStart=%s)", ErrDiskResizeCannotMoveBiosBootPartition,
					diskutils.HumanReadable(partition.Start*sectorSize),
					diskutils.HumanReadable(nextSector*sectorSize))
				return diskutils.PartitionTable{}, 0, err
			}

			newPartition.Start = mathutils.RoundUp(nextSector, alignmentSectors)
		}

		nextSector = newPartition.Start + newPartition.Size
		newPartitions[index] = newPartition
	}

	// GPT footer.
	nextSector += mathutils.DivRoundUp(int64(gptFooterSize), sectorSize)
	newDiskSize := uint64(mathutils.RoundUp(nextSector*sectorSize, alignmentSize))

	// Create new partition table.
	newPartitionTable := partitionTable
	newPartitionTable.Partitions = newPartitions

	return newPartitionTable, newDiskSize, nil
}

// Grow the size of a partition and the filesystem on that partition.
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

// Grow a filesystem.
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

// Grow an ext4 filesystem.
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

// Grow a xfs filesystem.
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

// Grow a btrfs filesystem.
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

// Find the partition for a given partition reference.
func resolvePartitionForResizeOp(resizePartConfig imagecustomizerapi.ResizePartition,
	partitionTable diskutils.PartitionTable, lastPartIndex int,
) (int, error) {
	switch resizePartConfig.Ref.IdType {
	case imagecustomizerapi.ResizePartitionRefTypeLast:
		return lastPartIndex, nil

	case imagecustomizerapi.ResizePartitionRefTypePartLabel, imagecustomizerapi.ResizePartitionRefTypeLabel,
		imagecustomizerapi.ResizePartitionRefTypePartUuid:

		matches := []int(nil)
		for i, partInfo := range partitionTable.Partitions {
			match := false
			switch resizePartConfig.Ref.IdType {
			case imagecustomizerapi.ResizePartitionRefTypePartLabel:
				match = partInfo.PartLabel == resizePartConfig.Ref.Id

			case imagecustomizerapi.ResizePartitionRefTypeLabel:
				match = partInfo.FileSystemLabel == resizePartConfig.Ref.Id

			case imagecustomizerapi.ResizePartitionRefTypePartUuid:
				match = partInfo.PartUuid == resizePartConfig.Ref.Id
			}

			if match {
				matches = append(matches, i)
			}
		}

		if len(matches) <= 0 {
			return 0, fmt.Errorf("%w (idType='%s', id='%s')", ErrDiskResizeFindPartitionFailed,
				resizePartConfig.Ref.IdType, resizePartConfig.Ref.Id)
		}

		if len(matches) > 1 {
			return 0, fmt.Errorf("%w (idType='%s', id='%s')", ErrDiskResizeFindTooManyPartitions,
				resizePartConfig.Ref.IdType, resizePartConfig.Ref.Id)
		}

		return matches[0], nil

	default:
		// This should be prevented by ResizePartition.IsValid().
		err := fmt.Errorf("no reference provided for partition resize")
		return 0, err
	}
}

// Finds the last partition on the disk.
// Note: Some distros (e.g. Ubuntu) don't have the partition table in the same order as the physical layout on disk.
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

// Calculate the new size of a partition based on the provided resize configuration.
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

// Create a new raw disk file, initially populating it with the GPT header and footer from an existing disk, and then
// changing the partition's starts and sizes (in the GPT header).
func createNewResizedDisk(source string, dest string, destDiskSize uint64, gptHeaderSize uint64, gptFooterSize uint64,
	newPartitionTable diskutils.PartitionTable,
) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Set the disk's size.
	err = destFile.Truncate(int64(destDiskSize))
	if err != nil {
		return fmt.Errorf("failed to resize destination disk:\n%w", err)
	}

	err = copyGptHeaderAndFooter(sourceFile, destFile, gptHeaderSize, gptFooterSize)
	if err != nil {
		return err
	}

	err = destFile.Close()
	if err != nil {
		return err
	}

	err = updatePartitionsStartAndSize(dest, newPartitionTable)
	if err != nil {
		return err
	}

	return nil
}

// Copy the GPT header and GPT footer from an existing disk to a new disk.
func copyGptHeaderAndFooter(sourceFile *os.File, destFile *os.File, gptHeaderSize uint64, gptFooterSize uint64) error {
	// Copy the GPT header.
	// Note: Go will use sendfile here to optimize the copy operation.
	_, err := io.Copy(destFile, io.LimitReader(sourceFile, int64(gptHeaderSize)))
	if err != nil {
		return fmt.Errorf("failed to copy the GPT header:\n%w", err)
	}

	// Copy the GPT footer.
	_, err = destFile.Seek(-int64(gptFooterSize), io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to the GPT footer in source disk:\n%w", err)
	}

	_, err = sourceFile.Seek(-int64(gptFooterSize), io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to the GPT footer in destination disk:\n%w", err)
	}

	// Note: Go will use sendfile here to optimize the copy operation.
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy the GPT footer:\n%w", err)
	}

	return nil
}

// Update all the partitions' start and size in a raw disk file.
// Does not move or copy the actual partition data.
// TODO: This is creating a new disk layout, instead of modifying the existing one. We need to use --partno. :-(
func updatePartitionsStartAndSize(path string, newPartitionTable diskutils.PartitionTable) error {
	// Build the sfdisk script.
	var sb strings.Builder
	fmt.Fprintf(&sb, "unit: sectors\n")
	fmt.Fprintf(&sb, "\n")

	for _, part := range newPartitionTable.Partitions {
		partNum, err := diskutils.GetPartitionNum(part.Path)
		if err != nil {
			return err
		}

		fmt.Fprintf(&sb, "%s%d : start=%d, size=%d\n", path, partNum, part.Start, part.Size)
	}

	sfdiskScript := sb.String()

	logger.Log.Debugf("sfdisk script:\n%s", sfdiskScript)

	// Format partition table.
	err := shell.NewExecBuilder("sfdisk", "--lock=no", path).
		Stdin(sfdiskScript).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("sfdisk failed:\n%w", err)
	}

	return nil
}
