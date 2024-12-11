// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type PreviewFeatures struct {
	Uki *Uki `yaml:"uki"`
}

func (p *PreviewFeatures) IsValid() error {
	var err error
	if p.Uki != nil {
		err = p.Uki.IsValid()
		if err != nil {
			return fmt.Errorf("invalid uki:\n%w", err)
		}
	}

	return nil
}
