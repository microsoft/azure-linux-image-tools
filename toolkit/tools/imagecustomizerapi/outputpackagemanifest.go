// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type OutputPackageManifest struct {
	Path string `yaml:"path" json:"path,omitempty"`
}

func (output *OutputPackageManifest) IsValid() error {
	if output.Path == "" {
		return fmt.Errorf("'path' must be specified and non-empty")
	}

	return nil
}
