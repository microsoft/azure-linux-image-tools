// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type SigningMethod struct {
	Ephemeral *SigningMethodEphemeral `yaml:"ephemeral" json:"ephemeral,omitempty"`
}

func (m *SigningMethod) IsValid() error {
	count := 0

	if m.Ephemeral != nil {
		err := m.Ephemeral.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'ephemeral' field:\n%w", err)
		}

		count += 1
	}

	if count != 1 {
		return fmt.Errorf("one and only one signing method must be specified")
	}

	return nil
}
