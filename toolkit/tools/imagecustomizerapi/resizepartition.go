// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ResizePartition struct {
	// The partition to resize.
	Ref ResizePartitionRef `yaml:"ref" json:"ref,omitempty"`
	// The amount of free space the partition's filesystem should have. If it is currently less than this amount, then
	// the partition and filesystem sizes are increased.
	FreeSpace *DiskSize `yaml:"freeSpace" json:"freeSpace,omitempty"`
	// The new size of the partition.
	Size *DiskSize `yaml:"size" json:"size,omitempty"`
}

func (r *ResizePartition) IsValid() error {
	if err := r.Ref.IsValid(); err != nil {
		return fmt.Errorf("invalid 'ref' value:\n%w", err)
	}

	if r.FreeSpace != nil {
		if err := r.FreeSpace.IsValid(); err != nil {
			return fmt.Errorf("invalid 'freeSpace' value:\n%w", err)
		}
	}

	if r.Size != nil {
		if err := r.Size.IsValid(); err != nil {
			return fmt.Errorf("invalid 'size' value:\n%w", err)
		}
	}

	if r.FreeSpace == nil && r.Size == nil {
		return fmt.Errorf("one of 'freeSpace' or 'size' must be specified")
	}

	if r.FreeSpace != nil && r.Size != nil {
		return fmt.Errorf("cannot specify both 'freeSpace' and 'size'")
	}

	return nil
}
