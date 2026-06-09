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
	PreviewFeatureReinitializeVerity PreviewFeature = "reinitialize-verity"

	// PreviewFeatureSnapshotTime enables support for snapshot-based package filtering.
	PreviewFeaturePackageSnapshotTime PreviewFeature = "package-snapshot-time"

	// PreviewFeatureKdumpBootFiles enables support for crash dump configuration.
	PreviewFeatureKdumpBootFiles PreviewFeature = "kdump-boot-files"

	// PreviewFeatureFedora enables support for Fedora images.
	PreviewFeatureFedora PreviewFeature = "fedora"

	// PreviewFeatureUbuntu enables support for Ubuntu images.
	PreviewFeatureUbuntu PreviewFeature = "ubuntu"

	// PreviewFeatureBaseConfigs enables support for base configuration.
	PreviewFeatureBaseConfigs PreviewFeature = "base-configs"

	// PreviewFeatureInputImageOci enables support for download OCI images.
	PreviewFeatureInputImageOci PreviewFeature = "input-image-oci"

	// PreviewFeatureOutputSelinuxPolicy enables extraction of SELinux policy contents.
	PreviewFeatureOutputSelinuxPolicy PreviewFeature = "output-selinux-policy"

	// PreviewFeatureBtrfs enables support for creating BTRFS file systems.
	PreviewFeatureBtrfs PreviewFeature = "btrfs"

	// PreviewFeatureCreate enables the create command for building new images from scratch.
	PreviewFeatureCreate PreviewFeature = "create"

	// PreviewFeatureAzureContainerLinux enables support for Azure Container Linux images.
	PreviewFeatureAzureContainerLinux PreviewFeature = "azure-container-linux"

	// PreviewFeatureUnsupportedDistroVersion allows distro versions that are not supported yet.
	PreviewFeatureUnsupportedDistroVersion PreviewFeature = "unsupported-distro-version"
)

func (pf PreviewFeature) IsValid() error {
	switch pf {
	case PreviewFeatureUki, PreviewFeatureOutputArtifacts, PreviewFeatureInjectFiles, PreviewFeatureReinitializeVerity,
		PreviewFeaturePackageSnapshotTime, PreviewFeatureKdumpBootFiles, PreviewFeatureFedora,
		PreviewFeatureUbuntu, PreviewFeatureBaseConfigs,
		PreviewFeatureInputImageOci, PreviewFeatureOutputSelinuxPolicy,
		PreviewFeatureBtrfs, PreviewFeatureCreate, PreviewFeatureAzureContainerLinux,
		PreviewFeatureUnsupportedDistroVersion:
		return nil
	default:
		return fmt.Errorf("invalid preview feature: %s", pf)
	}
}
