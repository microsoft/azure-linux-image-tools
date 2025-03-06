// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type Output struct {
	Image OutputImage `yaml:"image" json:"image,omitempty"`
}

func (o Output) IsValid() error {
	return o.Image.IsValid()
}
