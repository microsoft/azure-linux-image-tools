// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
)

// MergePreviewFeatures merges CLI preview features with config preview features.
// CLI features are added to config features, and duplicates are removed.
// The resulting slice maintains the order: config features first, then CLI features (excluding duplicates).
func MergePreviewFeatures(
	configFeatures []imagecustomizerapi.PreviewFeature,
	cliFeatures []imagecustomizerapi.PreviewFeature,
) []imagecustomizerapi.PreviewFeature {
	if len(cliFeatures) == 0 {
		return configFeatures
	}

	if len(configFeatures) == 0 {
		return cliFeatures
	}

	// Create a set of existing config features for quick lookup
	existingFeatures := make(map[imagecustomizerapi.PreviewFeature]struct{}, len(configFeatures))
	for _, feature := range configFeatures {
		existingFeatures[feature] = struct{}{}
	}

	// Start with config features, then add CLI features that aren't already present
	result := make([]imagecustomizerapi.PreviewFeature, len(configFeatures), len(configFeatures)+len(cliFeatures))
	copy(result, configFeatures)

	for _, feature := range cliFeatures {
		if _, exists := existingFeatures[feature]; !exists {
			result = append(result, feature)
			existingFeatures[feature] = struct{}{}
		}
	}

	return result
}

// StringsToPreviewFeatures converts a slice of strings to a slice of PreviewFeature.
// This function assumes the strings have already been validated (e.g., by the CLI parser).
func StringsToPreviewFeatures(features []string) []imagecustomizerapi.PreviewFeature {
	if len(features) == 0 {
		return nil
	}

	result := make([]imagecustomizerapi.PreviewFeature, len(features))
	for i, feature := range features {
		result[i] = imagecustomizerapi.PreviewFeature(feature)
	}
	return result
}
