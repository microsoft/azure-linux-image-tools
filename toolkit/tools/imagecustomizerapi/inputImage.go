// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type InputImage struct {
	Path string `yaml:"path" json:"path,omitempty"`
}

func (ii InputImage) IsValid() error {
	return nil
}
