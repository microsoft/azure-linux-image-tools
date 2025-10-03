// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"
)

type BaseConfig struct {
	Path string `yaml:"path" json:"path"`
}

type BaseConfigs []BaseConfig

func (b BaseConfig) IsValid() error {
	if strings.TrimSpace(b.Path) == "" {
		return fmt.Errorf("path must not be empty or whitespace")
	}
	return nil
}

func (b BaseConfigs) IsValid() error {
	for i, base := range b {
		if err := base.IsValid(); err != nil {
			return fmt.Errorf("invalid baseConfig item at index %d:\n%w", i, err)
		}
	}
	return nil
}
