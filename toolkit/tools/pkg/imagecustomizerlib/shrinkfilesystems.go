package imagecustomizerlib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
)

func shrinkFilesystems(imageLoopDevice string) error {
	logger.Log.Infof("Shrinking filesystems")

	// Get partition info
	diskPartitions, err := diskutils.GetDiskPartitions(imageLoopDevice)
	if err != nil {
		return err
	}

	sectorSize, _, err := diskutils.GetSectorSize(imageLoopDevice)
	if err != nil {
		return fmt.Errorf("failed to get disk (%s) sector size:\n%w", imageLoopDevice, err)
	}

	for _, diskPartition := range diskPartitions {
		if diskPartition.Type != "part" {
			continue
		}

		partitionLoopDevice := diskPartition.Path

		// Check if the filesystem type is supported
		fstype := diskPartition.FileSystemType
		if !supportedShrinkFsType(fstype) {
			logger.Log.Infof("Shrinking partition (%s): unsupported filesystem type (%s)", partitionLoopDevice, fstype)
			continue
		}

		logger.Log.Infof("Shrinking partition (%s)", partitionLoopDevice)

		fileSystemSizeInBytes := uint64(0)
		switch fstype {
		case "ext2", "ext3", "ext4":
			fileSystemSizeInBytes, err = shrinkExt4FileSystem(partitionLoopDevice, imageLoopDevice)
			if err != nil {
				return fmt.Errorf("failed to shrink %s filesystem (%s):\n%w", fstype, partitionLoopDevice, err)
			}

		default:
			continue
		}

		if fileSystemSizeInBytes == 0 {
			// The filesystem wasn't resized. So, there is no need to resize the partition.
			logger.Log.Infof("Filesystem is already at its min size (%s)", partitionLoopDevice)
			continue
		}

		fileSystemSizeInSectors := convertBytesToSectors(fileSystemSizeInBytes, sectorSize)

		err = resizePartition(partitionLoopDevice, imageLoopDevice, fileSystemSizeInSectors)
		if err != nil {
			return err
		}
	}
	return nil
}

func shrinkExt4FileSystem(partitionDevice string, diskDevice string) (uint64, error) {
	// Check the file system with e2fsck
	err := shell.ExecuteLive(true /*squashErrors*/, "e2fsck", "-fy", partitionDevice)
	if err != nil {
		return 0, fmt.Errorf("failed to check (%s) with e2fsck:\n%w", partitionDevice, err)
	}

	// Shrink the file system with resize2fs -M
	stdout, stderr, err := shell.Execute("flock", "--timeout", "5", diskDevice,
		"resize2fs", "-M", partitionDevice)
	if err != nil {
		return 0, fmt.Errorf("failed to resize (%s) with resize2fs (and flock):\n%v", partitionDevice, stderr)
	}

	// Find the new partition end value
	fileSystemSizeInBytes, err := getExt4FileSystemSizeInBytes(stdout, stderr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse new filesystem size from resize2fs output:\n%w", err)
	}

	return fileSystemSizeInBytes, nil
}

func resizePartition(partitionDevice string, diskDevice string, newSizeInSectors uint64) error {
	partitionNumber, err := getPartitionNum(partitionDevice)
	if err != nil {
		return err
	}

	// Resize the partition.
	sfdiskScript := fmt.Sprintf("unit: sectors\nsize=%d", newSizeInSectors)

	err = shell.NewExecBuilder("flock", "--timeout", "5", diskDevice, "sfdisk", "--lock=no",
		"-N", strconv.Itoa(partitionNumber), diskDevice).
		Stdin(sfdiskScript).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to resize partition (%s) with sfdisk (and flock):\n%w", partitionDevice, err)
	}

	// Changes to the partition table causes all of the disk's parition /dev nodes to be deleted and then
	// recreated. So, wait for that to finish.
	err = diskutils.RefreshPartitions(diskDevice)
	if err != nil {
		return fmt.Errorf("failed to wait for disk (%s) to update:\n%w", diskDevice, err)
	}

	return nil
}

// Get the filesystem size in bytes.
// Returns 0 if the resize was a no-op.
func getExt4FileSystemSizeInBytes(resize2fsStdout string, resize2fsStderr string) (uint64, error) {
	const resize2fsNopMessage = "Nothing to do!"
	if strings.Contains(resize2fsStderr, resize2fsNopMessage) {
		// Resize operation was a no-op.
		return 0, nil
	}

	// Example resize2fs output first line: "Resizing the filesystem on /dev/loop44p2 to 21015 (4k) blocks."
	re, err := regexp.Compile(`.*to (\d+) \((\d+)([a-zA-Z])\)`)
	if err != nil {
		return 0, fmt.Errorf("failed to compile regex:\n%w", err)
	}

	// Get the block count and block size
	match := re.FindStringSubmatch(resize2fsStdout)
	if match == nil {
		return 0, fmt.Errorf("failed to parse output of resize2fs:\nstdout:\n%s\nstderr:\n%s", resize2fsStdout,
			resize2fsStderr)
	}

	blockCount, err := strconv.ParseUint(match[1], 10, 64) // Example: 21015
	if err != nil {
		return 0, fmt.Errorf("failed to parse block count (%s):\n%w", match[1], err)
	}
	multiplier, err := strconv.ParseUint(match[2], 10, 64) // Example: 4
	if err != nil {
		return 0, fmt.Errorf("failed to parse multiplier for block size (%s):\n%w", match[2], err)
	}
	unit := match[3] // Example: 'k'

	// Calculate block size
	var blockSize uint64
	switch unit {
	case "k":
		blockSize = multiplier * diskutils.KiB
	default:
		return 0, fmt.Errorf("unrecognized unit (%s)", unit)
	}

	filesystemSizeInBytes := blockCount * uint64(blockSize)
	return filesystemSizeInBytes, nil
}

func convertBytesToSectors(sizeInBytes uint64, sectorSizeInBytes uint64) uint64 {
	sizeInSectors := sizeInBytes / sectorSizeInBytes
	rem := sizeInBytes % sectorSizeInBytes
	if rem != 0 {
		sizeInSectors += 1
	}

	return sizeInSectors
}

// Checks if the provided fstype is supported by shrink filesystems.
func supportedShrinkFsType(fstype string) (isSupported bool) {
	switch fstype {
	// Currently only support ext2, ext3, ext4 filesystem types
	case "ext2", "ext3", "ext4":
		return true
	default:
		return false
	}
}
