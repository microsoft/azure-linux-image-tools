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

	// When reinitialize is passthrough, kernels must not be specified (since we preserve existing UKIs).
	// For refresh mode or when reinitialize is not set, kernels must be specified.
	if u.Reinitialize == UkiReinitializePassthrough {
		if u.Kernels.Auto || len(u.Kernels.Kernels) > 0 {
			return fmt.Errorf("'kernels' must not be specified when 'reinitialize' is 'passthrough'")
		}
	} else {
		err = u.Kernels.IsValid()
		if err != nil {
			return fmt.Errorf("invalid uki kernels:\n%w", err)
		}
	}

	return nil
}
