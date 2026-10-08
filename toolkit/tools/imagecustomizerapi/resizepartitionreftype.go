// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ResizePartitionRefType string

const (
	ResizePartitionRefTypeLast      ResizePartitionRefType = "last"
	ResizePartitionRefTypePartLabel ResizePartitionRefType = "part-label"
	ResizePartitionRefTypeLabel     ResizePartitionRefType = "label"
	ResizePartitionRefTypePartUuid  ResizePartitionRefType = "part-uuid"
)

func (t ResizePartitionRefType) IsValid() error {
	switch t {
	case ResizePartitionRefTypeLast, ResizePartitionRefTypePartLabel, ResizePartitionRefTypeLabel,
		ResizePartitionRefTypePartUuid:

		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%s)", t)
	}
}
