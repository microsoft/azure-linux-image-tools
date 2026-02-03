package imagecustomizerlib

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

var (
	// Filesystem operation errors
	ErrFilesystemSectorSizeGet = NewImageCustomizerError("Filesystem:SectorSizeGet", "failed to get disk sector size")
	ErrFilesystemShrink        = NewImageCustomizerError("Filesystem:Shrink", "failed to shrink filesystem")
	ErrFilesystemE2fsckResize  = NewImageCustomizerError("Filesystem:E2fsckResize", "failed to check filesystem with e2fsck")
	ErrFilesystemResize2fs     = NewImageCustomizerError("Filesystem:Resize2fs", "failed to resize filesystem with resize2fs")
	ErrFilesystemTune2fs       = NewImageCustomizerError("Filesystem:Tune2fs", "failed to get filesystem info with tune2fs")
)

// shrinkFilesystemsHelper shrinks filesystems to minimize their size.
// When requireCoverage is true (used by convert subcommand), filesystems will only be shrunk
// if they completely cover their partition (filesystem size == partition size).
// When requireCoverage is false (used by customize subcommand), filesystems are always shrunk
// since IC controls the image creation and all filesystems should cover their partitions.
func shrinkFilesystemsHelper(ctx context.Context, buildImageFile string, readonlyPartUuids []string, requireCoverage bool) (map[string]uint64, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "shrink_filesystems")
	defer span.End()

	imageLoopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return nil, err
	}
	defer imageLoopback.Close()

	// Shrink the filesystems and capture original sizes.
	partitionOriginalSizes, err := shrinkFilesystems(imageLoopback.DevicePath(), readonlyPartUuids, requireCoverage)
	if err != nil {
		return nil, err
	}

	err = imageLoopback.CleanClose()
	if err != nil {
		return nil, err
	}

	return partitionOriginalSizes, nil
}

func shrinkFilesystems(imageLoopDevice string, readonlyPartUuids []string, requireCoverage bool) (map[string]uint64, error) {
	logger.Log.Infof("Shrinking filesystems")

	// Get partition info
	diskPartitions, err := diskutils.GetDiskPartitions(imageLoopDevice)
	if err != nil {
		return nil, err
	}

	// Capture original partition sizes before shrinking (partUuid -> size in bytes)
	partitionOriginalSizes := make(map[string]uint64)
	for _, diskPartition := range diskPartitions {
		if diskPartition.Type == "part" {
			partitionOriginalSizes[diskPartition.PartUuid] = diskPartition.SizeInBytes
		}
	}

	sectorSize, _, err := diskutils.GetSectorSize(imageLoopDevice)
	if err != nil {
		return nil, fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemSectorSizeGet, imageLoopDevice, err)
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

		readonly := slices.Contains(readonlyPartUuids, diskPartition.PartUuid)
		if readonly {
			logger.Log.Infof("Shrinking partition (%s): skipping read-only partition (%s)", partitionLoopDevice, fstype)
			continue
		}

		// When requireCoverage is true (convert subcommand), only shrink if filesystem
		// completely covers the partition. This is a safety check for arbitrary input images
		// where we can't guarantee what's in the gap between filesystem and partition end.
		if requireCoverage {
			covers, err := filesystemCoversPartition(partitionLoopDevice, fstype, diskPartition.SizeInBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to check filesystem coverage (device='%s'):\n%w", partitionLoopDevice, err)
			}
			if !covers {
				logger.Log.Infof("Shrinking partition (%s): skipping - filesystem does not cover entire partition", partitionLoopDevice)
				continue
			}
		}

		logger.Log.Infof("Shrinking partition (%s)", partitionLoopDevice)

		fileSystemSizeInBytes := uint64(0)
		switch fstype {
		case "ext2", "ext3", "ext4":
			fileSystemSizeInBytes, err = shrinkExt4FileSystem(partitionLoopDevice, imageLoopDevice)
			if err != nil {
				return nil, fmt.Errorf("%w (type='%s', device='%s'):\n%w", ErrFilesystemShrink, fstype, partitionLoopDevice, err)
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
			return nil, err
		}
	}
	return partitionOriginalSizes, nil
}

func shrinkExt4FileSystem(partitionDevice string, diskDevice string) (uint64, error) {
	// Check the file system with e2fsck
	err := shell.ExecuteLive(true /*squashErrors*/, "e2fsck", "-fy", partitionDevice)
	if err != nil {
		return 0, fmt.Errorf("%w (device='%s'):\n%w", ErrFilesystemE2fsckResize, partitionDevice, err)
	}

	// Shrink the file system with resize2fs -M
	stdout, stderr, err := shell.Execute("flock", "--timeout", "5", diskDevice,
		"resize2fs", "-M", partitionDevice)
	if err != nil {
		return 0, fmt.Errorf("%w (device='%s', stderr='%s'):\n%w", ErrFilesystemResize2fs, partitionDevice, stderr, err)
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

// filesystemCoversPartition checks if the filesystem completely covers the partition.
func filesystemCoversPartition(partitionDevice string, fstype string, partitionSizeInBytes uint64) (bool, error) {
	var filesystemSizeInBytes uint64
	var err error

	switch fstype {
	case "ext2", "ext3", "ext4":
		filesystemSizeInBytes, err = getExt4CurrentFilesystemSize(partitionDevice)
		if err != nil {
			return false, err
		}
	default:
		// For unsupported filesystem types, assume they don't cover (be conservative)
		return false, nil
	}

	covers := filesystemSizeInBytes == partitionSizeInBytes
	if !covers {
		logger.Log.Debugf("Filesystem coverage check: filesystem=%d bytes, partition=%d bytes, delta=%d bytes",
			filesystemSizeInBytes, partitionSizeInBytes, int64(partitionSizeInBytes)-int64(filesystemSizeInBytes))
	}
	return covers, nil
}

func getExt4CurrentFilesystemSize(partitionDevice string) (uint64, error) {
	stdout, stderr, err := shell.Execute("tune2fs", "-l", partitionDevice)
	if err != nil {
		return 0, fmt.Errorf("%w (device='%s', stderr='%s'):\n%w", ErrFilesystemTune2fs, partitionDevice, stderr, err)
	}

	// Parse "Block count:" line
	blockCountRegex := regexp.MustCompile(`(?m)^Block count:\s+(\d+)`)
	blockCountMatch := blockCountRegex.FindStringSubmatch(stdout)
	if blockCountMatch == nil {
		return 0, fmt.Errorf("failed to parse 'Block count' from tune2fs output:\n%s", stdout)
	}
	blockCount, err := strconv.ParseUint(blockCountMatch[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block count value (%s):\n%w", blockCountMatch[1], err)
	}

	// Parse "Block size:" line
	blockSizeRegex := regexp.MustCompile(`(?m)^Block size:\s+(\d+)`)
	blockSizeMatch := blockSizeRegex.FindStringSubmatch(stdout)
	if blockSizeMatch == nil {
		return 0, fmt.Errorf("failed to parse 'Block size' from tune2fs output:\n%s", stdout)
	}
	blockSize, err := strconv.ParseUint(blockSizeMatch[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block size value (%s):\n%w", blockSizeMatch[1], err)
	}

	filesystemSizeInBytes := blockCount * blockSize
	return filesystemSizeInBytes, nil
}
