// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type IdentifiedPartitionType string

const (
	IdentifiedPartitionTypePartLabel IdentifiedPartitionType = "part-label"
)

func (i IdentifiedPartitionType) IsValid() error {
	switch i {
	case IdentifiedPartitionTypePartLabel:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%s)", i)
	}
}
