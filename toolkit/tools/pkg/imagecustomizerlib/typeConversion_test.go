// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/stretchr/testify/assert"
)

func TestCalcIsBootPartition(t *testing.T) {
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/boot",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/boot/efi",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.True(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
	assert.False(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
		[]imagecustomizerapi.FileSystem{
			{
				MountPoint: &imagecustomizerapi.MountPoint{
					Path: "/boot",
				},
			},
		},
	))
	assert.False(t, calcIsBootPartition(
		imagecustomizerapi.FileSystem{
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/var",
			},
		},
		[]imagecustomizerapi.FileSystem{},
	))
}

func TestPartitionSettingsForBtrfsSubvolumes(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "btrfs-partition",
		Type:        imagecustomizerapi.FileSystemTypeBtrfs,
		Btrfs: &imagecustomizerapi.BtrfsConfig{
			Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
				{
					Path: "root",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/",
					},
				},
				{
					Path: "home",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/home",
					},
				},
			},
		},
	}

	settings, err := partitionSettingsForBtrfsSubvolumes(fileSystem)
	assert.NoError(t, err)

	expected := []configuration.PartitionSetting{
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "subvol=/root",
			MountPoint:      "/",
		},
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "subvol=/home",
			MountPoint:      "/home",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForBtrfsSubvolumesWithOptions(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "btrfs-partition",
		Type:        imagecustomizerapi.FileSystemTypeBtrfs,
		Btrfs: &imagecustomizerapi.BtrfsConfig{
			Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
				{
					Path: "root",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path:    "/",
						Options: "compress=zstd,noatime",
					},
				},
			},
		},
	}

	settings, err := partitionSettingsForBtrfsSubvolumes(fileSystem)
	assert.NoError(t, err)

	expected := []configuration.PartitionSetting{
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "subvol=/root,compress=zstd,noatime",
			MountPoint:      "/",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForBtrfsSubvolumesNoMountPoint(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "btrfs-partition",
		Type:        imagecustomizerapi.FileSystemTypeBtrfs,
		Btrfs: &imagecustomizerapi.BtrfsConfig{
			Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
				{
					Path: "snapshots", // No mount point
				},
				{
					Path: "root",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/",
					},
				},
			},
		},
	}

	settings, err := partitionSettingsForBtrfsSubvolumes(fileSystem)
	assert.NoError(t, err)

	// All subvolumes should be included, even without mount points (consistent with non-BTRFS behavior)
	expected := []configuration.PartitionSetting{
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "",
			MountPoint:      "",
		},
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "subvol=/root",
			MountPoint:      "/",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForFileSystemBtrfs(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "btrfs-partition",
		Type:        imagecustomizerapi.FileSystemTypeBtrfs,
		Btrfs: &imagecustomizerapi.BtrfsConfig{
			Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
				{
					Path: "root",
					MountPoint: &imagecustomizerapi.MountPoint{
						Path: "/",
					},
				},
			},
		},
	}

	settings, err := partitionSettingsForFileSystem(fileSystem)
	assert.NoError(t, err)

	expected := []configuration.PartitionSetting{
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "subvol=/root",
			MountPoint:      "/",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForFileSystemExt4(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "ext4-partition",
		Type:        imagecustomizerapi.FileSystemTypeExt4,
		MountPoint: &imagecustomizerapi.MountPoint{
			Path: "/",
		},
	}

	settings, err := partitionSettingsForFileSystem(fileSystem)
	assert.NoError(t, err)

	expected := []configuration.PartitionSetting{
		{
			ID:              "ext4-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountPoint:      "/",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForFileSystemExt4NoMountPoint(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "ext4-partition",
		Type:        imagecustomizerapi.FileSystemTypeExt4,
		MountPoint:  nil,
	}

	settings, err := partitionSettingsForFileSystem(fileSystem)
	assert.NoError(t, err)

	// Non-BTRFS filesystems should get a partition setting even without a mount point
	expected := []configuration.PartitionSetting{
		{
			ID:              "ext4-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountPoint:      "",
		},
	}
	assert.Equal(t, expected, settings)
}

func TestPartitionSettingsForBtrfsSubvolumesAllNoMountPoint(t *testing.T) {
	fileSystem := imagecustomizerapi.FileSystem{
		PartitionId: "btrfs-partition",
		Type:        imagecustomizerapi.FileSystemTypeBtrfs,
		Btrfs: &imagecustomizerapi.BtrfsConfig{
			Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
				{
					Path: "snapshots",
				},
				{
					Path: "data",
				},
			},
		},
	}

	settings, err := partitionSettingsForBtrfsSubvolumes(fileSystem)
	assert.NoError(t, err)

	// All subvolumes should be included even without mount points
	expected := []configuration.PartitionSetting{
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "",
			MountPoint:      "",
		},
		{
			ID:              "btrfs-partition",
			MountIdentifier: configuration.MountIdentifierPartUuid,
			MountOptions:    "",
			MountPoint:      "",
		},
	}
	assert.Equal(t, expected, settings)
}
