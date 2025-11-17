// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type Uki struct {
	Kernels      UkiKernels      `yaml:"kernels" json:"kernels"`
	Reinitialize UkiReinitialize `yaml:"reinitialize" json:"reinitialize"`
}

func (u *Uki) IsValid() error {
	err := u.Reinitialize.IsValid()
	if err != nil {
		return fmt.Errorf("invalid uki reinitialize:\n%w", err)
	}

	err = u.Kernels.IsValid()
	if err != nil {
		return fmt.Errorf("invalid uki kernels:\n%w", err)
	}

	return nil
}
