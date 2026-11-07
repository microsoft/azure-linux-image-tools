// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

var ErrAclUsrFsUuid = NewImageCustomizerError("Acl:UsrFsUuid", "failed to regenerate ACL /usr filesystem UUID")

func aclUsrUuidRegenerationEnabled(acl *imagecustomizerapi.Acl) bool {
	return acl != nil && acl.RegenerateUsrFsUuid != nil && *acl.RegenerateUsrFsUuid
}

func regenerateAclUsrFsUuid(connection *osImageConnection, verity []verityDeviceMetadata) error {
	if connection.Chroot() != nil {
		return fmt.Errorf("%w: image must be unmounted", ErrAclUsrFsUuid)
	}
	partitions, err := diskutils.GetDiskPartitions(connection.Loopback().DevicePath())
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclUsrFsUuid, err)
	}
	partition, err := aclUsrUuidPartition(connection.partitionsLayout, verity, partitions)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrAclUsrFsUuid, err)
	}

	logger.Log.Infof("Regenerating ACL /usr filesystem UUID")
	if _, err := resetFileSystemUuid(partition); err != nil {
		return fmt.Errorf("%w (partition='%s'):\n%w", ErrAclUsrFsUuid, partition.Path, err)
	}
	return nil
}

func aclUsrUuidPartition(layout []fstabEntryPartNum, verity []verityDeviceMetadata,
	partitions []diskutils.PartitionInfo,
) (diskutils.PartitionInfo, error) {
	partition, relativePath, err := getPartitionOfPath("/usr", partitions, layout)
	if err != nil {
		return diskutils.PartitionInfo{}, err
	}
	if relativePath != "." {
		return diskutils.PartitionInfo{}, fmt.Errorf("/usr must have its own partition")
	}
	if partition.FileSystemType != "btrfs" {
		return diskutils.PartitionInfo{}, fmt.Errorf("/usr must use Btrfs")
	}
	if partition.Mountpoint != "" {
		return diskutils.PartitionInfo{}, fmt.Errorf("/usr partition (%s) is still mounted", partition.Path)
	}
	count := 0
	for _, metadata := range verity {
		if metadata.name != imagecustomizerapi.VerityUsrDeviceName {
			continue
		}
		if metadata.dataPartUuid != partition.PartUuid {
			return diskutils.PartitionInfo{}, fmt.Errorf("/usr verity data partition does not match the active /usr")
		}
		if _, _, err := findPartition(imagecustomizerapi.MountIdentifierTypePartUuid, metadata.hashPartUuid, partitions); err != nil {
			return diskutils.PartitionInfo{}, fmt.Errorf("/usr verity hash partition is missing:\n%w", err)
		}
		count++
	}
	if count != 1 {
		return diskutils.PartitionInfo{}, fmt.Errorf("expected one /usr verity device, found %d", count)
	}
	return partition, nil
}
