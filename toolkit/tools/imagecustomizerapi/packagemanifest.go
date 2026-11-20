// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type PackageManifest struct {
	Mode PackageManifestMode `yaml:"mode" json:"mode"`
}

func (manifest *PackageManifest) IsValid() error {
	err := manifest.Mode.IsValid()
	if err != nil {
		return fmt.Errorf("invalid package manifest mode:\n%w", err)
	}

	return nil
}
