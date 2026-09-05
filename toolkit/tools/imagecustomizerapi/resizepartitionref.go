// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type ResizePartitionRef string

const (
	ResizePartitionRefLast ResizePartitionRef = "last"
)

func (r ResizePartitionRef) IsValid() error {
	switch r {
	case ResizePartitionRefLast:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%s)", r)
	}
}
