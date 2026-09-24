// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ResizeDisk struct {
	// The new size of the disk.
	DiskSize *DiskSize `yaml:"diskSize" json:"diskSize,omitempty"`
	// The partitions to resize.
	Partitions []ResizePartition `yaml:"partitions" json:"partitions,omitempty"`
}

func (r *ResizeDisk) IsValid() error {
	if r.DiskSize != nil {
		if err := r.DiskSize.IsValid(); err != nil {
			return fmt.Errorf("invalid 'diskSize' value:\n%w", err)
		}
	}

	for i, partition := range r.Partitions {
		if err := partition.IsValid(); err != nil {
			return fmt.Errorf("invalid 'partitions' value at index %d:\n%w", i, err)
		}
	}

	if len(r.Partitions) > 1 {
		return fmt.Errorf("resizing multiple partitions is not yet supported")
	}

	if len(r.Partitions) > 0 && r.DiskSize != nil {
		return fmt.Errorf("cannot specify both diskSize and partitions")
	}

	if len(r.Partitions) == 0 && r.DiskSize == nil {
		return fmt.Errorf("must specify either diskSize or partitions")
	}

	return nil
}
