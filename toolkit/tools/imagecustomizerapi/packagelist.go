// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

type PackageList struct {
	Packages []string `yaml:"packages" json:"packages,omitempty"`
}

func (s *PackageList) IsValid() error {
	return nil
}
