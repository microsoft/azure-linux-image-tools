// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"slices"
)

type PreviewFeature string

const (
	// PreviewFeatureUki enables the Unified Kernel Image (UKI) feature.
	PreviewFeatureUki PreviewFeature = "uki"

	// PreviewFeatureOutputArtifacts enables output of selected artifacts after image customization.
	PreviewFeatureOutputArtifacts PreviewFeature = "output-artifacts"

	// PreviewFeatureInjectFiles enables files injection into target partitions.
	PreviewFeatureInjectFiles PreviewFeature = "inject-files"

	// PreviewFeatureReinitializeVerity will reinitialize verity on verity partitions in the base image.
	PreviewFeatureReinitializeVerity PreviewFeature = "reinitialize-verity"

	// PreviewFeatureSnapshotTime enables support for snapshot-based package filtering.
	PreviewFeaturePackageSnapshotTime PreviewFeature = "package-snapshot-time"

	// PreviewFeatureKdumpBootFiles enables support for crash dump configuration.
	PreviewFeatureKdumpBootFiles PreviewFeature = "kdump-boot-files"

	// PreviewFeatureFedora42 enables support for Fedora 42 images.
	PreviewFeatureFedora42 PreviewFeature = "fedora-42"

	// PreviewFeatureUbuntu2204 enables support for Ubuntu 22.04 images.
	PreviewFeatureUbuntu2204 PreviewFeature = "ubuntu-22.04"

	// PreviewFeatureUbuntu2404 enables support for Ubuntu 24.04 images.
	PreviewFeatureUbuntu2404 PreviewFeature = "ubuntu-24.04"

	// PreviewFeatureBaseConfigs enables support for base configuration.
	PreviewFeatureBaseConfigs PreviewFeature = "base-configs"

	// PreviewFeatureInputImageOci enables support for download OCI images.
	PreviewFeatureInputImageOci PreviewFeature = "input-image-oci"

	// PreviewFeatureOutputSelinuxPolicy enables extraction of SELinux policy contents.
	PreviewFeatureOutputSelinuxPolicy PreviewFeature = "output-selinux-policy"

	// PreviewFeatureCosiCompression enables custom compression settings for COSI output.
	PreviewFeatureCosiCompression PreviewFeature = "cosi-compression"

	// PreviewFeatureBtrfs enables support for creating BTRFS file systems.
	PreviewFeatureBtrfs PreviewFeature = "btrfs"

	// PreviewFeatureCreate enables the create command for building new images from scratch.
	PreviewFeatureCreate PreviewFeature = "create"
)

// supportedPreviewFeatures is a sorted list of all valid preview features.
var supportedPreviewFeatures = []string{
	string(PreviewFeatureBaseConfigs),
	string(PreviewFeatureBtrfs),
	string(PreviewFeatureCosiCompression),
	string(PreviewFeatureCreate),
	string(PreviewFeatureFedora42),
	string(PreviewFeatureInjectFiles),
	string(PreviewFeatureInputImageOci),
	string(PreviewFeatureKdumpBootFiles),
	string(PreviewFeatureOutputArtifacts),
	string(PreviewFeatureOutputSelinuxPolicy),
	string(PreviewFeaturePackageSnapshotTime),
	string(PreviewFeatureReinitializeVerity),
	string(PreviewFeatureUbuntu2204),
	string(PreviewFeatureUbuntu2404),
	string(PreviewFeatureUki),
}

// SupportedPreviewFeatures returns all valid preview feature values.
func SupportedPreviewFeatures() []string {
	return supportedPreviewFeatures
}

func (pf PreviewFeature) IsValid() error {
	if !slices.Contains(SupportedPreviewFeatures(), string(pf)) {
		return fmt.Errorf("invalid preview feature: %s", pf)
	}
	return nil
}
