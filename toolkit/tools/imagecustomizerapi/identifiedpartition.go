// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type IdentifiedPartition struct {
	IdType IdentifiedPartitionType `yaml:"idType"`
	Id     string                  `yaml:"id"`
}

func (i *IdentifiedPartition) IsValid() error {
	// Check if IdType is valid
	if err := i.IdType.IsValid(); err != nil {
		return fmt.Errorf("invalid idType:\n%w", err)
	}

	// Check if Id is not empty
	if i.Id == "" {
		return fmt.Errorf("invalid id: empty string")
	}

	// Check Id format based on IdType
	switch i.IdType {
	case IdentifiedPartitionTypePartLabel:
		if err := isGPTNameValid(i.Id); err != nil {
			return fmt.Errorf("invalid id format for %s:\n%w", IdentifiedPartitionTypePartLabel, err)
		}
	}

	return nil
}
