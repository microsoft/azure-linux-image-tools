// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
)

var (
	// Filesystem check errors
	ErrFilesystemE2fsckCheck    = NewImageCustomizerError("FilesystemCheck:E2fsck", "failed to check filesystem with e2fsck")
	ErrFilesystemXfsRepairCheck = NewImageCustomizerError("FilesystemCheck:XfsRepair", "failed to check filesystem with xfs_repair")
	ErrFilesystemBtrfsCheck     = NewImageCustomizerError("FilesystemCheck:Btrfs", "failed to check filesystem with btrfs check")
	ErrFilesystemFsckCheck      = NewImageCustomizerError("FilesystemCheck:Fsck", "failed to check filesystem with fsck")
)

func checkFileSystems(ctx context.Context, rawImageFile string) error {
	logger.Log.Infof("Checking for file system errors")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "check_filesystems")
	defer span.End()

	imageLoopback, err := safeloopback.NewLoopback(rawImageFile)
	if err != nil {
		return err
	}
	defer imageLoopback.Close()

	err = checkFileSystemsHelper(imageLoopback.DevicePath())
	if err != nil {
		return err
	}

	err = imageLoopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func checkFileSystemsHelper(diskDevice string) error {
	// Get partitions info
	diskPartitions, err := diskutils.GetDiskPartitions(diskDevice)
	if err != nil {
		return err
	}

	errs := []error(nil)
	for _, diskPartition := range diskPartitions {
		if diskPartition.Type != "part" {
			// Skip the disk entry.
			continue
		}

		if diskPartition.FileSystemType == "" {
			// Skip partitions that don't have a known file system type (e.g. the BIOS boot partition).
			logger.Log.Debugf("Skipping file system check (%s)", diskPartition.Path)
			continue
		}

		err = checkFileSystem(diskPartition.FileSystemType, diskPartition.Path)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func checkFileSystemFile(fileSystemType string, path string) error {
	if fileSystemType == "" {
		// Skip partitions that don't have a known file system type (e.g. the BIOS boot partition).
		logger.Log.Debugf("Skipping file system check (%s)", path)
		return nil
	}

	loopback, err := safeloopback.NewLoopback(path)
	if err != nil {
		return err
	}
	defer loopback.Close()

	err = checkFileSystem(fileSystemType, loopback.DevicePath())
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func checkFileSystem(fileSystemType string, path string) error {
	logger.Log.Debugf("Check file system (%s) at (%s)", fileSystemType, path)

	// Check the file system for corruption.
	switch fileSystemType {
	case "ext2", "ext3", "ext4":
		// Add -f flag to force check to run even if the journal is marked as clean.
		err := shell.ExecuteLive(true /*squashErrors*/, "e2fsck", "-fn", path)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrFilesystemE2fsckCheck, path, err)
		}

	case "xfs":
		// The fsck.xfs tool doesn't do anything. So, call xfs_repair instead.
		err := shell.ExecuteLive(true /*squashErrors*/, "xfs_repair", "-n", path)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrFilesystemXfsRepairCheck, path, err)
		}

	case "btrfs":
		// Use btrfs check in read-only mode to check the filesystem.
		err := shell.ExecuteLive(true /*squashErrors*/, "btrfs", "check", "--readonly", path)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrFilesystemBtrfsCheck, path, err)
		}

	default:
		err := shell.ExecuteLive(true /*squashErrors*/, "fsck", "-n", path)
		if err != nil {
			return fmt.Errorf("%w (path='%s'):\n%w", ErrFilesystemFsckCheck, path, err)
		}
	}

	return nil
}
