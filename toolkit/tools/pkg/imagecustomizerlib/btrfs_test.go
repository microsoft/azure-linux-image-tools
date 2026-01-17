// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
)

func TestCreateBtrfsSubvolumes_EmptyFileSystems_Pass(t *testing.T) {
	err := createBtrfsSubvolumes(nil, nil, nil, "/tmp")
	assert.NoError(t, err)

	err = createBtrfsSubvolumes([]imagecustomizerapi.FileSystem{}, nil, nil, "/tmp")
	assert.NoError(t, err)
}

func TestCreateBtrfsSubvolumes_NoBtrfsConfig_Pass(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			PartitionId: "part1",
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
		},
	}

	partIDToDevPathMap := map[string]string{
		"part1": "/fake/device/path",
	}
	partIDToFsTypeMap := map[string]string{
		"part1": "ext4",
	}

	err := createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, "/tmp")
	assert.NoError(t, err)
}

func TestCreateBtrfsSubvolumes_NoSubvolumes_Pass(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			PartitionId: "part1",
			MountPoint: &imagecustomizerapi.MountPoint{
				Path: "/",
			},
			Btrfs: &imagecustomizerapi.BtrfsConfig{},
		},
	}

	partIDToDevPathMap := map[string]string{
		"part1": "/fake/device/path",
	}
	partIDToFsTypeMap := map[string]string{
		"part1": "btrfs",
	}

	err := createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, "/tmp")
	assert.NoError(t, err)
}

func TestCreateBtrfsSubvolumes_DeviceNotFound_Fail(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			PartitionId: "part1",
			Btrfs: &imagecustomizerapi.BtrfsConfig{
				Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
					{
						Path:       "root",
						MountPoint: &imagecustomizerapi.MountPoint{Path: "/"},
					},
				},
			},
		},
	}

	partIDToDevPathMap := map[string]string{}
	partIDToFsTypeMap := map[string]string{}

	err := createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find device path for partition (part1)")
}

func TestCreateBtrfsSubvolumes_FsTypeNotFound_Fail(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			PartitionId: "part1",
			Btrfs: &imagecustomizerapi.BtrfsConfig{
				Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
					{
						Path:       "root",
						MountPoint: &imagecustomizerapi.MountPoint{Path: "/"},
					},
				},
			},
		},
	}

	partIDToDevPathMap := map[string]string{
		"part1": "/fake/device/path",
	}
	partIDToFsTypeMap := map[string]string{}

	err := createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find filesystem type for partition (part1)")
}

func TestCreateBtrfsSubvolumes_NotBtrfsFs_Fail(t *testing.T) {
	fileSystems := []imagecustomizerapi.FileSystem{
		{
			PartitionId: "part1",
			Btrfs: &imagecustomizerapi.BtrfsConfig{
				Subvolumes: []imagecustomizerapi.BtrfsSubvolume{
					{
						Path:       "root",
						MountPoint: &imagecustomizerapi.MountPoint{Path: "/"},
					},
				},
			},
		},
	}

	partIDToDevPathMap := map[string]string{
		"part1": "/fake/device/path",
	}
	partIDToFsTypeMap := map[string]string{
		"part1": "ext4",
	}

	err := createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has subvolumes defined but filesystem type is (ext4), not btrfs")
}

func TestCreateBtrfsSubvolumesOnDevice_EmptyInput_Pass(t *testing.T) {
	err := createBtrfsSubvolumesOnDevice("/fake/device/path", nil, "/tmp")
	assert.NoError(t, err)

	err = createBtrfsSubvolumesOnDevice("/fake/device/path", []btrfsSubvolumeConfig{}, "/tmp")
	assert.NoError(t, err)
}

func TestSortBtrfsSubvolumesByDepth_EmptyInput_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{}
	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, 0, len(sorted))
}

func TestSortBtrfsSubvolumesByDepth_SingleElement_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{{Path: "root"}}
	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, input, sorted)
}

func TestSortBtrfsSubvolumesByDepth_UnsortedInput_Pass(t *testing.T) {
	input := []btrfsSubvolumeConfig{
		{Path: "root/var/lib/postgresql"},
		{Path: "root"},
		{Path: "home/user/documents/work"},
		{Path: "root/var"},
		{Path: "home"},
		{Path: "var/log"},
		{Path: "home/user"},
		{Path: "root/var/lib"},
	}

	// Sorted alphabetically by path. This ensures parent subvolumes are created before their children.
	expected := []btrfsSubvolumeConfig{
		{Path: "home"},
		{Path: "home/user"},
		{Path: "home/user/documents/work"},
		{Path: "root"},
		{Path: "root/var"},
		{Path: "root/var/lib"},
		{Path: "root/var/lib/postgresql"},
		{Path: "var/log"},
	}

	sorted := sortBtrfsSubvolumesByDepth(input)
	assert.Equal(t, expected, sorted)
}
