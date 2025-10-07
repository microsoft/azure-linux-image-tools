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

func (b BaseConfig) IsValid() error {
	if strings.TrimSpace(b.Path) == "" {
		return fmt.Errorf("path must not be empty or whitespace")
	}
	return nil
}
