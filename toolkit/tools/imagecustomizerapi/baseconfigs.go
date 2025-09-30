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

func (b BaseConfigs) IsValid() error {
	for _, base := range b {
		if strings.TrimSpace(base.Path) == "" {
			return fmt.Errorf("baseConfigs entry has empty or whitespace-only path")
		}
	}
	return nil
}
