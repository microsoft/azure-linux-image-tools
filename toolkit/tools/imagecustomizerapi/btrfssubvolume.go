// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"
)

// BtrfsSubvolume defines a single BTRFS subvolume.
type BtrfsSubvolume struct {
	// Path is the path to the subvolume within the top-level subvolume (subvolid=5), relative to its root.
	Path string `yaml:"path" json:"path"`
	// MountPoint is the optional mount configuration for this subvolume.
	MountPoint *MountPoint `yaml:"mountPoint" json:"mountPoint,omitempty"`
	// Quota is the optional quota settings for this subvolume.
	Quota *BtrfsQuotaConfig `yaml:"quota" json:"quota,omitempty"`
}

// IsValid validates the BtrfsSubvolume configuration.
func (s *BtrfsSubvolume) IsValid() error {
	if err := validateSubvolumePath(s.Path); err != nil {
		return fmt.Errorf("invalid path:\n%w", err)
	}

	if s.MountPoint != nil {
		if err := s.MountPoint.IsValid(); err != nil {
			return fmt.Errorf("invalid mountPoint:\n%w", err)
		}

		// Validate that subvol= and subvolid= are not in the options
		if err := validateBtrfsMountOptions(s.MountPoint.Options); err != nil {
			return fmt.Errorf("invalid mountPoint.options:\n%w", err)
		}
	}

	if s.Quota != nil {
		if err := s.Quota.IsValid(); err != nil {
			return fmt.Errorf("invalid quota:\n%w", err)
		}
	}

	return nil
}

// validateSubvolumePath validates a BTRFS subvolume path.
func validateSubvolumePath(subvolPath string) error {
	if subvolPath == "" {
		return fmt.Errorf("path must not be empty")
	}

	if strings.HasPrefix(subvolPath, "/") {
		return fmt.Errorf("path must not start with '/'")
	}

	if strings.HasSuffix(subvolPath, "/") {
		return fmt.Errorf("path must not end with '/'")
	}

	// Check for invalid path components
	parts := strings.Split(subvolPath, "/")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("path must not contain double slashes")
		}
		if part == ".." {
			return fmt.Errorf("path must not contain '..' components")
		}
		if part == "." {
			return fmt.Errorf("path must not contain '.' components")
		}
	}

	return nil
}
