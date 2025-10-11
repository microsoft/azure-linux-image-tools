// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type InputImage struct {
	Path string    `yaml:"path" json:"path,omitempty"`
	Oci  *OciImage `yaml:"oci" json:"oci,omitempty"`
}

func (ii *InputImage) IsValid() error {
	if ii.Path != "" && ii.Oci != nil {
		return fmt.Errorf("cannot specify both 'path' and 'oci'")
	}

	return nil
}
