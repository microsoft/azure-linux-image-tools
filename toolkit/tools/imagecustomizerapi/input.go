// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type Input struct {
	Image InputImage `yaml:"image" json:"image,omitempty"`
}

func (i Input) IsValid() error {
	return i.Image.IsValid()
}
