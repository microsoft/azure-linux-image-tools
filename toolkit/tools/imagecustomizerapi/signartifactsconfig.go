// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type SignArtifactsConfig struct {
	SigningMethod SigningMethod `yaml:"signingMethod" json:"signingMethod,omitempty"`
}

func (c *SignArtifactsConfig) IsValid() error {
	err := c.SigningMethod.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'signingMethod' field:\n%w", err)
	}

	return nil
}
