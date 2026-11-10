// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verityFileSystemTestStorage(t *testing.T, fsType, mountPath string, verity, inline bool) imagecustomizerapi.Storage {
	t.Helper()
	storage := imagecustomizerapi.Storage{
		BootType: imagecustomizerapi.BootTypeEfi,
		Disks: []imagecustomizerapi.Disk{{
			PartitionTableType: imagecustomizerapi.PartitionTableTypeGpt,
			Partitions: []imagecustomizerapi.Partition{
				{
					Id: "esp", Type: imagecustomizerapi.PartitionTypeESP,
					Size: imagecustomizerapi.PartitionSize{
						Type: imagecustomizerapi.PartitionSizeTypeExplicit, Size: 64 * diskutils.MiB,
					},
				},
				{
					Id: "data",
					Size: imagecustomizerapi.PartitionSize{
						Type: imagecustomizerapi.PartitionSizeTypeExplicit, Size: 512 * diskutils.MiB,
					},
				},
				{
					Id: "hash",
					Size: imagecustomizerapi.PartitionSize{
						Type: imagecustomizerapi.PartitionSizeTypeExplicit, Size: 16 * diskutils.MiB,
					},
				},
			},
		}},
		Verity: []imagecustomizerapi.Verity{{
			Id: "protected", Name: "root", DataDeviceId: "data", HashDeviceId: "hash",
		}},
		FileSystems: []imagecustomizerapi.FileSystem{
			{DeviceId: "esp", Type: imagecustomizerapi.FileSystemTypeFat32,
				MountPoint: &imagecustomizerapi.MountPoint{Path: "/boot/efi"}},
			{DeviceId: "protected", Type: imagecustomizerapi.FileSystemType(fsType),
				MountPoint: &imagecustomizerapi.MountPoint{Path: mountPath, Options: "ro"}},
		},
	}
	if mountPath == "/usr" {
		storage.Verity[0].Name = "usr"
	}
	if inline {
		storage.Verity[0].HashDeviceId = "data"
	}
	if !verity {
		storage.Verity = nil
		storage.FileSystems[1].DeviceId = "data"
	}
	require.NoError(t, storage.IsValid())
	return storage
}

// Check validation and propagation independently of filesystem type or mount path.
func TestPartitionToImagerLogicalBlockSize(t *testing.T) {
	for _, fsType := range []string{"xfs", "ext4", "btrfs"} {
		for _, test := range []struct {
			name      string
			mountPath string
			verity    bool
			inline    bool
			expected  uint64
		}{
			{"plain-root", "/", false, false, 0},
			{"plain-usr", "/usr", false, false, 0},
			{"verity-root", "/", true, false, imagecustomizerapi.DefaultVerityDataBlockSize},
			{"verity-usr", "/usr", true, false, imagecustomizerapi.DefaultVerityDataBlockSize},
			{"inline-root", "/", true, true, imagecustomizerapi.DefaultVerityDataBlockSize},
			{"inline-usr", "/usr", true, true, imagecustomizerapi.DefaultVerityDataBlockSize},
		} {
			t.Run(fmt.Sprintf("%s/%s", fsType, test.name), func(t *testing.T) {
				storage := verityFileSystemTestStorage(t, fsType, test.mountPath, test.verity, test.inline)
				assert.Equal(t, test.expected, storage.FileSystems[1].LogicalBlockSize)
				disk, err := diskConfigToImager(storage.Disks[0], storage.FileSystems)
				require.NoError(t, err)
				assert.Equal(t, test.expected, disk.Partitions[1].LogicalBlockSize)
				assert.Zero(t, disk.Partitions[0].LogicalBlockSize)
				assert.Zero(t, disk.Partitions[2].LogicalBlockSize)
			})
		}
	}
}
