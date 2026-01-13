// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

// InjectFilesConfig defines the list of files to be injected into partitions.
type InjectFilesConfig struct {
	// InjectFiles lists the files to inject into target partitions.
	InjectFiles []InjectArtifactMetadata `yaml:"injectFiles" json:"injectFiles,omitempty"`

	// PreviewFeatures lists preview features required to enable this config.
	PreviewFeatures []PreviewFeature `yaml:"previewFeatures,omitempty" json:"previewFeatures,omitempty"`
}

func (ifc *InjectFilesConfig) IsValid() error {
	if !slices.Contains(ifc.PreviewFeatures, PreviewFeatureInjectFiles) {
		return fmt.Errorf("the 'inject-files' feature is currently in preview; please add 'inject-files' to 'previewFeatures' to enable it")
	}

	for i, feature := range ifc.PreviewFeatures {
		if err := feature.IsValid(); err != nil {
			return fmt.Errorf("invalid 'previewFeatures' item at index %d:\n%w", i, err)
		}
	}

	for idx, entry := range ifc.InjectFiles {
		err := entry.IsValid()
		if err != nil {
			return fmt.Errorf("injectFiles[%d] is invalid:\n%w", idx, err)
		}
	}

	return nil
}
