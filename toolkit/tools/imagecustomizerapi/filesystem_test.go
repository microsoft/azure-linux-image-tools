// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileSystemIsValid_EmptyDeviceId_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "", // Invalid: empty
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'deviceId' value: must not be empty")
}

func TestFileSystemIsValid_InvalidType_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemType("ntfs"),
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid fileSystem (disk0-part1) 'type' value")
}

func TestFileSystemIsValid_BtrfsConfigWithNonBtrfsType_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeExt4,
		Btrfs:    &BtrfsConfig{},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'btrfs' configuration is only valid for 'btrfs' filesystems")
}

func TestFileSystemIsValid_InvalidBtrfsConfig_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		Btrfs: &BtrfsConfig{
			Subvolumes: []BtrfsSubvolume{
				{
					Path: "", // Invalid: empty
				},
			},
		},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'btrfs' configuration")
}

func TestFileSystemIsValid_BtrfsSubvolumesWithMountPoint_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		MountPoint: &MountPoint{
			Path: "/",
		},
		Btrfs: &BtrfsConfig{
			Subvolumes: []BtrfsSubvolume{
				{
					Path: "root",
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
			},
		},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err,
		"'mountPoint' cannot be set when 'btrfs.subvolumes' is non-empty")
}

func TestFileSystemIsValid_InvalidMountPoint_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeExt4,
		MountPoint: &MountPoint{
			Path:    "/data",
			Options: "ro noatime", // Invalid: contains space
		},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'mountPoint' value")
}

func TestFileSystemIsValid_MountPointWithNoType_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeNone,
		MountPoint: &MountPoint{
			Path: "/data",
		},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "filesystem with 'mountPoint' must have a 'type'")
}

func TestFileSystemIsValid_BtrfsInvalidMountOptions_Fail(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		MountPoint: &MountPoint{
			Path:    "/data",
			Options: "subvol=root",
		},
	}
	err := fs.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'mountPoint.options' for 'btrfs' filesystem")
}

func TestFileSystemIsValid_MinimalValid_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}

func TestFileSystemIsValid_WithMountPoint_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeExt4,
		MountPoint: &MountPoint{
			Path: "/data",
		},
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}

func TestFileSystemIsValid_BtrfsWithEmptyConfig_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		Btrfs:    &BtrfsConfig{},
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}

func TestFileSystemIsValid_BtrfsWithSubvolumes_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		Btrfs: &BtrfsConfig{
			Subvolumes: []BtrfsSubvolume{
				{
					Path: "root",
					MountPoint: &MountPoint{
						Path: "/",
					},
				},
				{
					Path: "home",
					MountPoint: &MountPoint{
						Path: "/home",
					},
				},
			},
		},
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}

func TestFileSystemIsValid_BtrfsWithMountPointNoSubvolumes_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		MountPoint: &MountPoint{
			Path:    "/data",
			Options: "compress=zstd",
		},
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}

func TestFileSystemIsValid_BtrfsEmptySubvolumesWithMountPoint_Pass(t *testing.T) {
	fs := FileSystem{
		DeviceId: "disk0-part1",
		Type:     FileSystemTypeBtrfs,
		MountPoint: &MountPoint{
			Path: "/data",
		},
		Btrfs: &BtrfsConfig{
			Subvolumes: []BtrfsSubvolume{}, // Empty slice is allowed
		},
	}
	err := fs.IsValid()
	assert.NoError(t, err)
}
