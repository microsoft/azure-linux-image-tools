// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// Acl holds the narrow, ACL-only configuration for growing Azure Container Linux's standard,
// well-known partitions to explicit target sizes.
//
// This is an internal, preview-gated API. It only ever enlarges ACL's existing, sealed
// partitions; it never reorders, adds, removes, or reformats them. It is only valid for ACL
// target images. Additional standard partitions (e.g. root, oem) may be added in the future.
type Acl struct {
	// Usr grows ACL's /usr partition(s) (USR-A and USR-B, kept the same size).
	Usr *AclPartitionGrow `yaml:"usr" json:"usr,omitempty"`
	// Esp grows ACL's EFI system partition (mounted at /boot on ACL).
	Esp *AclPartitionGrow `yaml:"esp" json:"esp,omitempty"`
}

// AclPartitionGrow describes the desired grown size of an ACL standard partition.
type AclPartitionGrow struct {
	// Size is the target size to grow the partition to. Growing only: requesting a size smaller
	// than the current partition size is an error; requesting the current size is a no-op.
	Size DiskSize `yaml:"size" json:"size,omitempty"`
}

func (a *Acl) IsValid() error {
	if a.Usr == nil && a.Esp == nil {
		return fmt.Errorf("'acl' must specify at least one partition to grow (e.g. 'usr' or 'esp')")
	}

	if a.Usr != nil {
		if err := a.Usr.IsValid(); err != nil {
			return fmt.Errorf("invalid 'acl.usr':\n%w", err)
		}
	}

	if a.Esp != nil {
		if err := a.Esp.IsValid(); err != nil {
			return fmt.Errorf("invalid 'acl.esp':\n%w", err)
		}
	}

	return nil
}

func (p *AclPartitionGrow) IsValid() error {
	if p.Size <= 0 {
		return fmt.Errorf("'size' must be specified and greater than 0")
	}

	return p.Size.IsValid()
}
