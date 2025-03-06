// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type OutputImage struct {
	Path string `yaml:"path" json:"path,omitempty"`
}

func (oi OutputImage) IsValid() error {
	return nil
}
