// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// btrfsSubvolumeConfig defines a single BTRFS subvolume to create.
type btrfsSubvolumeConfig struct {
	// Path is the subvolume path relative to the top-level subvolume (e.g., "root", "root/home").
	Path string
	// ReferencedLimit is the maximum total space the subvolume can use (in bytes). 0 means no limit.
	ReferencedLimit uint64
	// ExclusiveLimit is the maximum unshared space the subvolume can use (in bytes). 0 means no limit.
	ExclusiveLimit uint64
}

// createBtrfsSubvolumes creates BTRFS subvolumes for any file systems that have them defined.
func createBtrfsSubvolumes(fileSystems []imagecustomizerapi.FileSystem, partIDToDevPathMap map[string]string,
	partIDToFsTypeMap map[string]string, buildDir string,
) error {
	for _, fs := range fileSystems {
		if fs.Btrfs == nil || len(fs.Btrfs.Subvolumes) == 0 {
			continue
		}

		// Get the device path for this partition.
		partitionId := fs.PartitionId
		devicePath, ok := partIDToDevPathMap[partitionId]
		if !ok {
			return fmt.Errorf("failed to find device path for partition (%s)", partitionId)
		}

		// Verify this is a BTRFS filesystem.
		fsType, ok := partIDToFsTypeMap[partitionId]
		if !ok {
			return fmt.Errorf("failed to find filesystem type for partition (%s)", partitionId)
		}
		if fsType != "btrfs" {
			return fmt.Errorf("partition (%s) has subvolumes defined but filesystem type is (%s), not btrfs",
				partitionId, fsType)
		}

		// Convert to btrfsSubvolumeConfig.
		subvolumes := make([]btrfsSubvolumeConfig, 0, len(fs.Btrfs.Subvolumes))
		for _, sv := range fs.Btrfs.Subvolumes {
			subvolumeConfig := btrfsSubvolumeConfig{
				Path: sv.Path,
			}

			// Set quota limits if defined. 0 means no limit as non-nil limits must be positive due to validation.
			if sv.Quota != nil {
				if sv.Quota.ReferencedLimit != nil {
					subvolumeConfig.ReferencedLimit = uint64(*sv.Quota.ReferencedLimit)
				}
				if sv.Quota.ExclusiveLimit != nil {
					subvolumeConfig.ExclusiveLimit = uint64(*sv.Quota.ExclusiveLimit)
				}
			}

			subvolumes = append(subvolumes, subvolumeConfig)
		}

		// Create the subvolumes.
		err := createBtrfsSubvolumesOnDevice(devicePath, subvolumes, buildDir)
		if err != nil {
			return fmt.Errorf("failed to create BTRFS subvolumes for partition (%s):\n%w", partitionId, err)
		}
	}

	return nil
}

// createBtrfsSubvolumesOnDevice creates BTRFS subvolumes on a filesystem.
// It temporarily mounts the top-level subvolume, creates the subvolumes and parent directories,
// enables quotas if needed, and unmounts.
func createBtrfsSubvolumesOnDevice(devicePath string, subvolumes []btrfsSubvolumeConfig, buildDir string) error {
	if len(subvolumes) == 0 {
		return nil
	}

	logger.Log.Infof("Creating BTRFS subvolumes on %s", devicePath)

	// Create a temporary mount point for the top-level subvolume
	tempMountDir, err := os.MkdirTemp(buildDir, "btrfs-toplevel-")
	if err != nil {
		return fmt.Errorf("failed to create temp mount directory:\n%w", err)
	}
	defer os.RemoveAll(tempMountDir)

	// Mount the top-level subvolume (subvolid=5)
	mount, err := safemount.NewMount(devicePath, tempMountDir, "btrfs", 0, "subvolid=5", false)
	if err != nil {
		return fmt.Errorf("failed to mount top-level subvolume:\n%w", err)
	}
	defer mount.Close()

	// Create subvolumes, used to distinguish subvolumes from regular directories.
	subvolumeSet := make(map[string]bool)
	for _, subvol := range subvolumes {
		subvolumeSet[subvol.Path] = true
	}

	// Sort subvolumes by depth to ensure parents are created before children.
	sortedSubvolumes := sortBtrfsSubvolumesByDepth(subvolumes)

	for _, subvol := range sortedSubvolumes {
		err := createBtrfsSubvolume(tempMountDir, subvol.Path, subvolumeSet)
		if err != nil {
			return fmt.Errorf("failed to create subvolume '%s':\n%w", subvol.Path, err)
		}
	}

	// Enable quotas and set limits if any subvolumes have quota settings
	hasQuotas := false
	for _, subvol := range sortedSubvolumes {
		if subvol.ReferencedLimit > 0 || subvol.ExclusiveLimit > 0 {
			hasQuotas = true
			break
		}
	}

	if hasQuotas {
		err := enableBtrfsQuotas(tempMountDir)
		if err != nil {
			return fmt.Errorf("failed to enable BTRFS quotas:\n%w", err)
		}

		for _, subvol := range sortedSubvolumes {
			if subvol.ReferencedLimit > 0 || subvol.ExclusiveLimit > 0 {
				err := setBtrfsQuotaLimits(tempMountDir, subvol.Path, subvol.ReferencedLimit, subvol.ExclusiveLimit)
				if err != nil {
					return fmt.Errorf("failed to set quota for subvolume '%s':\n%w", subvol.Path, err)
				}
			}
		}
	}

	// Unmount
	err = mount.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to unmount top-level subvolume:\n%w", err)
	}

	logger.Log.Infof("Successfully created %d BTRFS subvolume(s)", len(subvolumes))
	return nil
}

// sortBtrfsSubvolumesByDepth returns a new slice of subvolumes sorted by path depth (shallower first).
// This ensures parent subvolumes are created before their children.
func sortBtrfsSubvolumesByDepth(subvolumes []btrfsSubvolumeConfig) []btrfsSubvolumeConfig {
	sorted := make([]btrfsSubvolumeConfig, len(subvolumes))
	copy(sorted, subvolumes)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.Count(sorted[i].Path, "/") < strings.Count(sorted[j].Path, "/")
	})
	return sorted
}

// createBtrfsSubvolume creates a single subvolume, creating parent directories as needed.
func createBtrfsSubvolume(mountDir, subvolPath string, subvolumeSet map[string]bool) error {
	fullPath := filepath.Join(mountDir, subvolPath)

	// Create parent directories if they don't exist and are not subvolumes
	parentPath := filepath.Dir(subvolPath)
	if parentPath != "." {
		parentParts := strings.Split(parentPath, "/")
		currentPath := ""
		for _, part := range parentParts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
			}

			fullParentPath := filepath.Join(mountDir, currentPath)

			// Check if this path already exists
			_, err := os.Stat(fullParentPath)
			if err == nil {
				// Path exists, continue
				continue
			}

			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to stat '%s':\n%w", fullParentPath, err)
			}

			// Skip subvolumes that will be created later
			if subvolumeSet[currentPath] {
				continue
			}

			logger.Log.Debugf("Creating directory: %s", fullParentPath)
			err = os.MkdirAll(fullParentPath, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create directory '%s':\n%w", fullParentPath, err)
			}
		}
	}

	// Create the subvolume
	logger.Log.Debugf("Creating subvolume: %s", subvolPath)
	err := shell.ExecuteLive(false, "btrfs", "subvolume", "create", fullPath)
	if err != nil {
		return fmt.Errorf("btrfs subvolume create failed:\n%w", err)
	}

	return nil
}

// enableBtrfsQuotas enables quota groups on a BTRFS filesystem.
func enableBtrfsQuotas(mountDir string) error {
	logger.Log.Debug("Enabling BTRFS quotas")
	err := shell.ExecuteLive(false, "btrfs", "quota", "enable", mountDir)
	if err != nil {
		return fmt.Errorf("btrfs quota enable failed:\n%w", err)
	}
	return nil
}

// setBtrfsQuotaLimits sets quota limits on a BTRFS subvolume.
func setBtrfsQuotaLimits(mountDir, subvolPath string, referencedLimit, exclusiveLimit uint64) error {
	fullPath := filepath.Join(mountDir, subvolPath)

	if referencedLimit > 0 {
		logger.Log.Debugf("Setting referenced limit %d on subvolume %s", referencedLimit, subvolPath)
		limitArg := fmt.Sprintf("%d", referencedLimit)
		err := shell.ExecuteLive(false, "btrfs", "qgroup", "limit", limitArg, fullPath)
		if err != nil {
			return fmt.Errorf("btrfs qgroup limit (referenced) failed:\n%w", err)
		}
	}

	if exclusiveLimit > 0 {
		logger.Log.Debugf("Setting exclusive limit %d on subvolume %s", exclusiveLimit, subvolPath)
		limitArg := fmt.Sprintf("%d", exclusiveLimit)
		err := shell.ExecuteLive(false, "btrfs", "qgroup", "limit", "-e", limitArg, fullPath)
		if err != nil {
			return fmt.Errorf("btrfs qgroup limit (exclusive) failed:\n%w", err)
		}
	}

	return nil
}
