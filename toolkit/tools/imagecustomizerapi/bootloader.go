// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type BootLoader struct {
	Reset ResetBootLoaderType `yaml:"reset"`
}

func (b *BootLoader) IsValid() error {
	err := b.Reset.IsValid()
	if err != nil {
		return fmt.Errorf("invalid bootloader reset:\n%w", err)
	}

	return nil
}
