// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type Input struct {
	Image InputImage `yaml:"image" json:"image,omitempty"`
}

func (i *Input) IsValid() error {
	err := i.Image.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'image' field:\n%w", err)
	}

	return nil
}
