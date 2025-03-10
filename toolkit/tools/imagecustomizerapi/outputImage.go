// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type OutputImage struct {
	Path   string          `yaml:"path" json:"path,omitempty"`
	Format ImageFormatType `yaml:"format" json:"format,omitempty"`
}

func (oi OutputImage) IsValid() error {
	if err := oi.Format.IsValid(); err != nil {
		return fmt.Errorf("invalid 'format' field:\n%w", err)
	}

	return nil
}
