// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

type InjectFilesYaml struct {
	InjectFiles []InjectArtifactMetadata `yaml:"injectFiles" json:"injectFiles,omitempty"`
}

func (i *InjectFilesYaml) IsValid() error {
	for idx, entry := range i.InjectFiles {
		if entry.Source == "" || entry.Destination == "" {
			return fmt.Errorf("injectFiles[%d] has empty source or destination", idx)
		}
		if entry.Partition.Id == "" {
			return fmt.Errorf("injectFiles[%d] has empty partition id", idx)
		}
		if err := entry.Partition.MountIdType.IsValid(); err != nil {
			return fmt.Errorf("injectFiles[%d] has invalid partition mount id type: %w", idx, err)
		}
	}
	return nil
}
