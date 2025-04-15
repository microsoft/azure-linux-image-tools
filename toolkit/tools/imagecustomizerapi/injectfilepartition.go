// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type InjectFilePartition struct {
	MountIdType MountIdentifierType `yaml:"idType" json:"idType,omitempty"`
	Id          string              `yaml:"id" json:"id,omitempty"`
}

func (ifp *InjectFilePartition) IsValid() error {
	if ifp.Id == "" {
		return fmt.Errorf("partition id is empty")
	}

	err := ifp.MountIdType.IsValid()
	if err != nil {
		return fmt.Errorf("invalid mount id type:\n%w", err)
	}

	return nil
}
