// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type Output struct {
	Path string `yaml:"path" json:"path,omitempty"`
}

func (o Output) IsValid() error {
	return nil
}
