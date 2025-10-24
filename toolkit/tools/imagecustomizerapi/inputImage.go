// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type InputImage struct {
	Path       string           `yaml:"path" json:"path,omitempty"`
	Oci        *OciImage        `yaml:"oci" json:"oci,omitempty"`
	AzureLinux *AzureLinuxImage `yaml:"azureLinux" json:"azureLinux,omitempty"`
}

func (ii *InputImage) IsValid() error {
	count := 0
	if ii.Path != "" {
		count++
	}
	if ii.Oci != nil {
		count++
	}
	if ii.AzureLinux != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("must only specify one of 'path', 'oci', and 'azureLinux'")
	}

	if ii.Oci != nil {
		err := ii.Oci.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'oci' field:\n%w", err)
		}
	}

	if ii.AzureLinux != nil {
		err := ii.AzureLinux.IsValid()
		if err != nil {
			return fmt.Errorf("invalid 'azureLinux' field:\n%w", err)
		}
	}

	return nil
}
