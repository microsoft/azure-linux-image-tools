// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

type SignArtifactsConfig struct {
	PreviewFeatures []PreviewFeature   `yaml:"previewFeatures" json:"previewFeatures,omitempty"`
	Input           SignArtifactsInput `yaml:"input" json:"input,omitempty"`
	SigningMethod   SigningMethod      `yaml:"signingMethod" json:"signingMethod,omitempty"`
}

func (c *SignArtifactsConfig) IsValid() error {
	for i, feature := range c.PreviewFeatures {
		if err := feature.IsValid(); err != nil {
			return fmt.Errorf("invalid 'previewFeatures' item at index %d:\n%w", i, err)
		}
	}

	err := c.Input.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'input' field:\n%w", err)
	}

	err = c.SigningMethod.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'signingMethod' field:\n%w", err)
	}

	if !slices.Contains(c.PreviewFeatures, PreviewFeatureSignArtifacts) {
		return fmt.Errorf("the '%s' preview feature must be enabled to sign-artifacts command",
			PreviewFeatureSignArtifacts)
	}

	return nil
}
