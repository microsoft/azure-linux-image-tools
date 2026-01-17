// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// BtrfsQuotaConfig defines quota settings for a BTRFS subvolume.
type BtrfsQuotaConfig struct {
	// ReferencedLimit is the maximum total space the subvolume can use ("referenced" limit).
	ReferencedLimit *DiskSize `yaml:"referencedLimit" json:"referencedLimit,omitempty"`
	// ExclusiveLimit is the maximum unshared space the subvolume can use ("exclusive" limit).
	ExclusiveLimit *DiskSize `yaml:"exclusiveLimit" json:"exclusiveLimit,omitempty"`
}

// IsValid validates the BtrfsQuotaConfig configuration.
func (q *BtrfsQuotaConfig) IsValid() error {
	if q.ReferencedLimit != nil && *q.ReferencedLimit <= 0 {
		return fmt.Errorf("referencedLimit value (%d) must be a positive non-zero number", *q.ReferencedLimit)
	}

	if q.ExclusiveLimit != nil && *q.ExclusiveLimit <= 0 {
		return fmt.Errorf("exclusiveLimit value (%d) must be a positive non-zero number", *q.ExclusiveLimit)
	}

	return nil
}
