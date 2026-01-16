// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"path"
	"strings"
)

// BtrfsConfig defines BTRFS-specific filesystem configuration.
type BtrfsConfig struct {
	// Subvolumes is a list of subvolumes to create within the filesystem.
	Subvolumes []BtrfsSubvolume `yaml:"subvolumes" json:"subvolumes,omitempty"`
}

// IsValid validates the BtrfsConfig configuration.
func (b *BtrfsConfig) IsValid() error {
	seenPaths := make(map[string]bool)
	for i, subvolume := range b.Subvolumes {
		err := subvolume.IsValid()
		if err != nil {
			return fmt.Errorf("invalid subvolume at index %d:\n%w", i, err)
		}

		if seenPaths[subvolume.Path] {
			return fmt.Errorf("invalid subvolume at index %d:\nduplicate path (%s)", i, subvolume.Path)
		}
		seenPaths[subvolume.Path] = true
	}

	if err := validateNoSubvolumeMountPointLoops(b.Subvolumes); err != nil {
		return err
	}

	return nil
}

// validateNoSubvolumeMountPointLoops checks for filesystem loops caused by subvolume path and mount point inversions.
//
// A loop occurs when:
//
//   - Subvolume A's path is an ancestor of subvolume B's path (B is nested under A)
//   - Subvolume B's mount point is an ancestor of subvolume A's mount point (A mounts under B)
//
// Example: subvol path "var" mounted at "/var/log", subvol path "var/log" mounted at "/var".
// This creates: /var -> /var/log -> /var/log/log -> ... (infinite loop).
func validateNoSubvolumeMountPointLoops(subvolumes []BtrfsSubvolume) error {
	for i, subvolA := range subvolumes {
		if subvolA.MountPoint == nil {
			continue
		}

		for j, subvolB := range subvolumes {
			if i == j || subvolB.MountPoint == nil {
				continue
			}

			mountPointA := path.Clean(subvolA.MountPoint.Path)
			mountPointB := path.Clean(subvolB.MountPoint.Path)
			aMountedUnderB := strings.HasPrefix(mountPointA, mountPointB+"/")
			bNestedUnderA := strings.HasPrefix(subvolB.Path, subvolA.Path+"/")

			if bNestedUnderA && aMountedUnderB {
				return fmt.Errorf(
					"subvolume mount point loop detected: subvolume '%s' (mounted at '%s') contains nested "+
						"subvolume '%s' (mounted at '%s'), but the mount points are inverted creating a "+
						"filesystem loop",
					subvolA.Path, subvolA.MountPoint.Path, subvolB.Path, subvolB.MountPoint.Path)
			}
		}
	}
	return nil
}
