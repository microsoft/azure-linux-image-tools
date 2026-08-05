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

	if r.FreeSpace == nil {
		return fmt.Errorf("the 'freeSpace' field must be specified")
	}

	return nil
}
