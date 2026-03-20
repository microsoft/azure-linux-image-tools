// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ResizeDisk struct {
	// The partitions to resize.
	Partitions []ResizePartition `yaml:"partitions" json:"partitions,omitempty"`
}

func (r *ResizeDisk) IsValid() error {
	for i, partition := range r.Partitions {
		if err := partition.IsValid(); err != nil {
			return fmt.Errorf("invalid 'partitions' value at index %d:\n%w", i, err)
		}
	}

	if len(r.Partitions) > 1 {
		return fmt.Errorf("resizing multiple partitions is not supported")
	}

	return nil
}
