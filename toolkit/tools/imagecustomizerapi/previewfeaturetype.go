// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

type PreviewFeature string

const (
	// PreviewFeatureUki enables the Unified Kernel Image (UKI) feature.
	PreviewFeatureUki PreviewFeature = "uki"

	// PreviewFeatureOutputArtifacts enables output of selected artifacts after image customization.
	PreviewFeatureOutputArtifacts PreviewFeature = "output-artifacts"

	// PreviewFeatureInjectFiles enables files injection into target partitions.
	PreviewFeatureInjectFiles PreviewFeature = "inject-files"

	// PreviewFeatureReinitializeVerity will reinitialize verity on verity partitions in the base image.
	PreviewFeatureReinitializeVerity = "reinitialize-verity"
)

func (pf PreviewFeature) IsValid() error {
	switch pf {
	case PreviewFeatureUki, PreviewFeatureOutputArtifacts, PreviewFeatureInjectFiles:
		return nil
	default:
		return fmt.Errorf("invalid preview feature: %s", pf)
	}
}
