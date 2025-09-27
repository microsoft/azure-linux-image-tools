// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// PartitionSetting holds the mounting information for each partition.
type PartitionSetting struct {
	RemoveDocs       bool            `json:"RemoveDocs"`
	ID               string          `json:"ID"`
	MountIdentifier  MountIdentifier `json:"MountIdentifier"`
	MountOptions     string          `json:"MountOptions"`
	MountPoint       string          `json:"MountPoint"`
	OverlayBaseImage string          `json:"OverlayBaseImage"`
	RdiffBaseImage   string          `json:"RdiffBaseImage"`
}

// FindMountpointPartitionSetting will search a list of partition settings for the partition setting
// corresponding to a mount point.
func FindMountpointPartitionSetting(partitionSettings []PartitionSetting, mountPoint string) (partitionSetting *PartitionSetting) {
	for _, p := range partitionSettings {
		if p.MountPoint == mountPoint {
			// We want to reference the actual object in the slice
			return &p
		}
	}
	return nil
}
