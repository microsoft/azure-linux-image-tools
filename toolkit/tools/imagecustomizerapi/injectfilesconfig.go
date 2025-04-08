// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// InjectFilesConfig defines the list of files to be injected into partitions.
type InjectFilesConfig struct {
	InjectFiles []InjectArtifactMetadata `yaml:"injectFiles" json:"injectFiles,omitempty"`
}

// IsValid verifies that all entries in the InjectFilesConfig are valid.
func (i *InjectFilesConfig) IsValid() error {
	for idx, entry := range i.InjectFiles {
		if entry.Source == "" || entry.Destination == "" {
			return fmt.Errorf("injectFiles[%d] has empty source or destination", idx)
		}
		if entry.Partition.Id == "" {
			return fmt.Errorf("injectFiles[%d] has empty partition id", idx)
		}
		if err := entry.Partition.MountIdType.IsValid(); err != nil {
			return fmt.Errorf("injectFiles[%d] has invalid partition mount id type:\n%w", idx, err)
		}
	}
	return nil
}
