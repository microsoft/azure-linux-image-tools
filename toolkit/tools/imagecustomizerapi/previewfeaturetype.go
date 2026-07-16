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

	// PreviewFeatureDistroVersion enables support for distros and distro versions that are still in
	// preview (e.g. Fedora, Ubuntu, Azure Container Linux, and Azure Linux 4.0).
	PreviewFeatureDistroVersion PreviewFeature = "preview-distro-version"

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

	// PreviewFeatureUnsupportedDistroVersion allows distro versions that are not supported yet.
	PreviewFeatureUnsupportedDistroVersion PreviewFeature = "unsupported-distro-version"

	// PreviewFeatureToolsDir enables support for specifying a tools directory.
	PreviewFeatureToolsDir PreviewFeature = "tools-dir"

	// PreviewFeatureRemovePackageManager enables support for the '.os.package.removePackageManager' API.
	PreviewFeatureRemovePackageManager PreviewFeature = "remove-package-manager"
)

func (pf PreviewFeature) IsValid() error {
	switch pf {
	case PreviewFeatureUki, PreviewFeatureOutputArtifacts, PreviewFeatureInjectFiles, PreviewFeatureReinitializeVerity,
		PreviewFeaturePackageSnapshotTime, PreviewFeatureKdumpBootFiles, PreviewFeatureDistroVersion,
		PreviewFeatureBaseConfigs, PreviewFeatureInputImageOci, PreviewFeatureOutputSelinuxPolicy, PreviewFeatureBtrfs,
		PreviewFeatureCreate, PreviewFeatureUnsupportedDistroVersion, PreviewFeatureToolsDir,
		PreviewFeatureRemovePackageManager:
		return nil
	default:
		return fmt.Errorf("invalid preview feature: %s", pf)
	}
}
