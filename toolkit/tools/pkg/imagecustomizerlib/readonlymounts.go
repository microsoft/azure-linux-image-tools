// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"iter"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

func getMostSpecificPath[K, V any](targetPath string, items iter.Seq2[K, V], pathFn func(V) string) (K, string, bool) {
	var mostSpecificIndex K
	mostSpecificPath := ""
	mostSpecificRelativePath := ""
	found := false

	for k, v := range items {
		path := pathFn(v)
		relativePath, err := filepath.Rel(path, targetPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, "../") {
			// Path is not relative to the mount.
			continue
		}

		if !found || len(path) > len(mostSpecificPath) {
			mostSpecificIndex = k
			mostSpecificPath = path
			mostSpecificRelativePath = relativePath
			found = true
		}
	}

	return mostSpecificIndex, mostSpecificRelativePath, found
}

func isPathOnReadOnlyMount(chrootPath string, imageChroot *safechroot.Chroot) bool {
	mounts := imageChroot.GetMountPoints()
	mountIndex, _, found := getMostSpecificPath(chrootPath, slices.All(mounts),
		func(mountPoint *safechroot.MountPoint) string {
			return mountPoint.GetTarget()
		})
	if !found {
		return false
	}

	mostSpecificMount := mounts[mountIndex]
	mountIsReadOnly := (mostSpecificMount.GetFlags() & unix.MS_RDONLY) != 0
	return mountIsReadOnly
}

func getPartitionOfPath(targetPath string, diskPartitions []diskutils.PartitionInfo,
	partUuidToFstabEntry map[string]diskutils.FstabEntry,
) (diskutils.PartitionInfo, string, error) {
	partUuid, relativePath, found := getMostSpecificPath(targetPath, maps.All(partUuidToFstabEntry),
		func(entry diskutils.FstabEntry) string {
			return entry.Target
		})
	if !found {
		return diskutils.PartitionInfo{}, "", fmt.Errorf("failed to find fstab entry for path (path='%s')", targetPath)
	}

	partitionInfo, found := sliceutils.FindValueFunc(diskPartitions,
		func(info diskutils.PartitionInfo) bool {
			return info.PartUuid == partUuid
		})
	if !found {
		return diskutils.PartitionInfo{}, "", fmt.Errorf("failed to partition for part UUID (partuuid='%s', path='%s)",
			partUuid, targetPath)
	}

	return partitionInfo, relativePath, nil
}
