// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type BootLoader struct {
	ResetType ResetBootLoaderType `yaml:"resetType"`
}

func (b *BootLoader) IsValid() error {
	err := b.ResetType.IsValid()
	if err != nil {
		return fmt.Errorf("invalid bootloader reset type:\n%w", err)
	}

	return nil
}
