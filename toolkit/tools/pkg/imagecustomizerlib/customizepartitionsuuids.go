// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"go.opentelemetry.io/otel"
)

var (
	// Partition UUID errors
	ErrPartitionUuidResetFilesystem   = NewImageCustomizerError("PartitionUUID:ResetFilesystem", "failed to reset filesystem UUID")
	ErrPartitionUuidUpdate            = NewImageCustomizerError("PartitionUUID:Update", "failed to update partition UUID")
	ErrPartitionE2fsckCheck           = NewImageCustomizerError("PartitionUUID:E2fsckCheck", "e2fsck check failed for partition")
	ErrPartitionVfatIdGenerate        = NewImageCustomizerError("PartitionUUID:VfatIdGenerate", "failed to generate VFAT ID")
	ErrResetPartitionIdOnVerityImage  = NewImageCustomizerError("PartitionUUID:ResetPartitionIdOnVerityImage", "resetting partition IDs on a verity-enabled image is not implemented")
	ErrPartitionUnsupportedFilesystem = NewImageCustomizerError("PartitionUUID:UnsupportedFilesystem", "unsupported filesystem for UUID reset")
)

func resetPartitionsUuids(ctx context.Context, buildImageFile string, buildDir string) error {
	logger.Log.Infof("Resetting partition UUIDs")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "reset_partitions_uuids")
	defer span.End()

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return err
	}
	defer loopback.Close()

	partitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	// Update the UUIDs.
	newUuids := make([]string, len(partitions))
	for i, partition := range partitions {
		if partition.Type != "part" {
			continue
		}

		newUuid, err := resetFileSystemUuid(partition)
		if err != nil {
			return fmt.Errorf("%w (partition='%s', type='%s'):\n%w", ErrPartitionUuidResetFilesystem, partition.Path, partition.FileSystemType, err)
		}

		newUuids[i] = newUuid
	}

	// Update the PARTUUIDs.
	newPartUuids := make([]string, len(partitions))
	for i, partition := range partitions {
		if partition.Type != "part" {
			continue
		}

		newPartUuid, err := resetPartitionUuid(loopback.DevicePath(), i)
		if err != nil {
			return fmt.Errorf("%w (partition='%s'):\n%w", ErrPartitionUuidUpdate, partition.Path, err)
		}

		newPartUuids[i] = newPartUuid
	}

	// Wait for the partition table updates to be processed.
	err = diskutils.WaitForDiskDevice(loopback.DevicePath())
	if err != nil {
		return err
	}

	// Fix /etc/fstab file.
	err = fixPartitionUuidsInFstabFile(partitions, newUuids, newPartUuids, buildDir)
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func resetFileSystemUuid(partition diskutils.PartitionInfo) (string, error) {
	newUuid := ""
	switch partition.FileSystemType {
	case "btrfs":
		newUuid = uuid.NewString()
		err := shell.ExecuteLive(true /*squashErrors*/, "btrfstune", "-U", newUuid, partition.Path)
		if err != nil {
			return "", err
		}

	case "ext2", "ext3", "ext4":
		// tune2fs requires you to run 'e2fsck -f' first.
		err := shell.ExecuteLive(true /*squashErrors*/, "e2fsck", "-fy", partition.Path)
		if err != nil {
			return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrPartitionE2fsckCheck, partition.Path, err)
		}

		newUuid = uuid.NewString()
		err = shell.ExecuteLive(true /*squashErrors*/, "tune2fs", "-U", newUuid, partition.Path)
		if err != nil {
			return "", err
		}

	case "xfs":
		newUuid = uuid.NewString()
		err := shell.ExecuteLive(true /*squashErrors*/, "xfs_admin", "-U", newUuid, partition.Path)
		if err != nil {
			return "", err
		}

	case "vfat":
		newUuidBytes := make([]byte, 4)
		_, err := rand.Read(newUuidBytes)
		if err != nil {
			return "", fmt.Errorf("%w:\n%w", ErrPartitionVfatIdGenerate, err)
		}

		newUuid = hex.EncodeToString(newUuidBytes)
		err = shell.ExecuteLive(true /*squashErrors*/, "fatlabel", "--volume-id", partition.Path, newUuid)
		if err != nil {
			return "", err
		}

		// Change the UUID string format to match what is expected by fstab.
		newUuid = strings.ToUpper(newUuid)
		newUuid = newUuid[:4] + "-" + newUuid[4:]

	case "DM_verity_hash":
		// Resetting partition IDs on a disk with verity would require updating the kernel command-line args.
		// This is probably doable, just not implemented yet.
		return "", ErrResetPartitionIdOnVerityImage

	default:
		return "", fmt.Errorf("%w (type='%s')", ErrPartitionUnsupportedFilesystem, partition.FileSystemType)
	}

	return newUuid, nil
}

func resetPartitionUuid(device string, partNum int) (string, error) {
	newUuid := uuid.NewString()
	err := shell.ExecuteLive(true /*squashErrors*/, "sfdisk", "--part-uuid", device, strconv.Itoa(partNum), newUuid)
	if err != nil {
		return "", err
	}

	return newUuid, nil
}

func fixPartitionUuidsInFstabFile(partitions []diskutils.PartitionInfo, newUuids []string, newPartUuids []string,
	buildDir string,
) error {
	rootfsPartition, rootfsPath, err := findRootfsPartition(partitions, buildDir)
	if err != nil {
		return err
	}

	// Mount the rootfs partition.
	tmpDir := filepath.Join(buildDir, tmpPartitionDirName)
	partitionMount, err := safemount.NewMount(rootfsPartition.Path, tmpDir, rootfsPartition.FileSystemType, 0, "", true)
	if err != nil {
		return err
	}
	defer partitionMount.Close()

	// Read the existing fstab file.
	fsTabFilePath := filepath.Join(partitionMount.Target(), rootfsPath, "etc/fstab")

	fstabEntries, err := diskutils.ReadFstabFile(fsTabFilePath)
	if err != nil {
		return err
	}

	// Fix the fstab entries.
	for i, fstabEntry := range fstabEntries {
		// Ignore special partitions.
		if isSpecialPartition(fstabEntry) {
			continue
		}

		mountIdType, mountId, err := parseExtendedSourcePartition(fstabEntry.Source)
		if err != nil {
			return err
		}

		switch mountIdType {
		case ExtendedMountIdentifierTypeUuid, ExtendedMountIdentifierTypePartUuid:

		default:
			// fstab entry doesn't need to be changed.
			continue
		}

		// Find the partition.
		// Note: The 'partitions' list was collected before all the changes were made. So, the fstab entries will still
		// match the values in the `partitions` list.
		_, partitionIndex, err := findPartitionHelper(imagecustomizerapi.MountIdentifierType(mountIdType), mountId,
			partitions)
		if err != nil {
			return err
		}

		// Create a new value for the source.
		newSource := fstabEntry.Source
		switch mountIdType {
		case ExtendedMountIdentifierTypeUuid:
			newSource = fmt.Sprintf("UUID=%s", newUuids[partitionIndex])

		case ExtendedMountIdentifierTypePartUuid:
			newSource = fmt.Sprintf("PARTUUID=%s", newPartUuids[partitionIndex])
		}

		logger.Log.Debugf("Fix fstab: (%s) to (%s)", fstabEntry.Source, newSource)
		fstabEntries[i].Source = newSource
	}

	// Write the updated fstab entries back to the fstab file.
	err = diskutils.WriteFstabFile(fstabEntries, fsTabFilePath)
	if err != nil {
		return err
	}

	err = partitionMount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
