// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
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
	PreviewFeatureReinitializeVerity = "reinitialize-verity"

	// PreviewFeatureSnapshotTime enables support for snapshot-based package filtering.
	PreviewFeaturePackageSnapshotTime PreviewFeature = "package-snapshot-time"

	// PreviewFeatureKdumpBootFiles enables support for crash dump configuration.
	PreviewFeatureKdumpBootFiles PreviewFeature = "kdump-boot-files"

	// PreviewFeatureFedora42 enables support for Fedora 42 images.
	PreviewFeatureFedora42 PreviewFeature = "fedora-42"

	// PreviewFeatureBaseConfigs enables support for base configuration.
	PreviewFeatureBaseConfigs PreviewFeature = "base-configs"

	// PreviewFeatureInputImageOci enables support for download OCI images.
	PreviewFeatureInputImageOci PreviewFeature = "input-image-oci"

	// PreviewFeatureOutputSelinuxPolicy enables extraction of SELinux policy contents.
	PreviewFeatureOutputSelinuxPolicy PreviewFeature = "output-selinux-policy"

	// PreviewFeatureOutputSelinuxPolicy enables the sign-artifacts command.
	PreviewFeatureSignArtifacts PreviewFeature = "sign-artifacts"
)

func (pf PreviewFeature) IsValid() error {
	switch pf {
	case PreviewFeatureUki, PreviewFeatureOutputArtifacts, PreviewFeatureInjectFiles, PreviewFeaturePackageSnapshotTime,
		PreviewFeatureKdumpBootFiles, PreviewFeatureFedora42, PreviewFeatureBaseConfigs, PreviewFeatureInputImageOci,
		PreviewFeatureOutputSelinuxPolicy, PreviewFeatureSignArtifacts:
		return nil
	default:
		return fmt.Errorf("invalid preview feature: %s", pf)
	}
}
