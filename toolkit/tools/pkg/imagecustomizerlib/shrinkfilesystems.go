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

		partitionNumber, err := getPartitionNum(partitionLoopDevice)
		if err != nil {
			return err
		}

		// Check the file system with e2fsck
		err = shell.ExecuteLive(true /*squashErrors*/, "e2fsck", "-fy", partitionLoopDevice)
		if err != nil {
			return fmt.Errorf("failed to check (%s) with e2fsck:\n%w", partitionLoopDevice, err)
		}

		// Shrink the file system with resize2fs -M
		stdout, stderr, err := shell.Execute("flock", "--timeout", "5", imageLoopDevice,
			"resize2fs", "-M", partitionLoopDevice)
		if err != nil {
			return fmt.Errorf("failed to resize (%s) with resize2fs (and flock):\n%v", partitionLoopDevice, stderr)
		}

		// Find the new partition end value
		filesystemSizeInSectors, err := getFilesystemSizeInSectors(stdout, stderr, imageLoopDevice)
		if err != nil {
			return fmt.Errorf("failed to parse new filesystem size:\n%w", err)
		}

		if filesystemSizeInSectors < 0 {
			// Filesystem wasn't resized. So, there is no need to resize the partition.
			logger.Log.Infof("Filesystem is already at its min size (%s)", partitionLoopDevice)
			continue
		}

		// Resize the partition with parted resizepart
		sfdiskScript := fmt.Sprintf("unit: sectors\nsize=%d", filesystemSizeInSectors)

		err = shell.NewExecBuilder("flock", "--timeout", "5", imageLoopDevice, "sfdisk", "--lock=no",
			"-N", strconv.Itoa(partitionNumber), imageLoopDevice).
			Stdin(sfdiskScript).
			LogLevel(logrus.DebugLevel, logrus.WarnLevel).
			ErrorStderrLines(1).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to resize partition (%s) with sfdisk (and flock):\n%v", partitionLoopDevice, stderr)
		}

		// Changes to the partition table causes all of the disk's parition /dev nodes to be deleted and then
		// recreated. So, wait for that to finish.
		err = diskutils.WaitForDiskDevice(imageLoopDevice)
		if err != nil {
			return fmt.Errorf("failed to wait for disk (%s) to update:\n%w", imageLoopDevice, err)
		}
	}
	return nil
}

// Get the filesystem size in sectors.
// Returns -1 if the resize was a no-op.
func getFilesystemSizeInSectors(resize2fsStdout string, resize2fsStderr string, imageLoopDevice string,
) (filesystemSizeInSectors int, err error) {
	const resize2fsNopMessage = "Nothing to do!"
	if strings.Contains(resize2fsStderr, resize2fsNopMessage) {
		// Resize operation was a no-op.
		return -1, nil
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

	blockCount, err := strconv.Atoi(match[1]) // Example: 21015
	if err != nil {
		return 0, fmt.Errorf("failed to parse block count (%s):\n%w", match[1], err)
	}
	multiplier, err := strconv.Atoi(match[2]) // Example: 4
	if err != nil {
		return 0, fmt.Errorf("failed to parse multiplier for block size (%s):\n%w", match[2], err)
	}
	unit := match[3] // Example: 'k'

	// Calculate block size
	var blockSize int
	const KiB = 1024 // kibibyte in bytes
	switch unit {
	case "k":
		blockSize = multiplier * KiB
	default:
		return 0, fmt.Errorf("unrecognized unit (%s)", unit)
	}

	filesystemSizeInBytes := blockCount * blockSize

	// Get the sector size
	logicalSectorSize, _, err := diskutils.GetSectorSize(imageLoopDevice)
	if err != nil {
		return 0, fmt.Errorf("failed to get sector size:\n%w", err)
	}
	sectorSizeInBytes := int(logicalSectorSize) // cast from uint64 to int

	filesystemSizeInSectors = filesystemSizeInBytes / sectorSizeInBytes
	if filesystemSizeInBytes%sectorSizeInBytes != 0 {
		filesystemSizeInSectors++
	}

	return filesystemSizeInSectors, nil
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
