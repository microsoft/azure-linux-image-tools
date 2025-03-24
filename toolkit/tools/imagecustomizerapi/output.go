// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type Output struct {
	Image     OutputImage `yaml:"image" json:"image,omitempty"`
	Artifacts *Artifacts  `yaml:"artifacts" json:"artifacts,omitempty"`
}

func (o Output) IsValid() error {
	if err := o.Image.IsValid(); err != nil {
		return fmt.Errorf("invalid 'image' field:\n%w", err)
	}

	if o.Artifacts != nil {
		if err := o.Artifacts.IsValid(); err != nil {
			return fmt.Errorf("invalid 'artifacts' field:\n%w", err)
		}
	}

	return nil
}
