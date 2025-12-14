// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type Uki struct {
	Mode UkiMode `yaml:"mode" json:"mode"`
}

func (u *Uki) IsValid() error {
	err := u.Mode.IsValid()
	if err != nil {
		return fmt.Errorf("invalid uki mode:\n%w", err)
	}

	return nil
}
