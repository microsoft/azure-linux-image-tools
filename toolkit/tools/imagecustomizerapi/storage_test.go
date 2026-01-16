// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/ptrutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStorageIsValidCoreEfi(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "rootfs",
					Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValidDuplicatePartitionID(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid filesystem item at index 1")
	assert.ErrorContains(t, err, "invalid 'deviceId'")
	assert.ErrorContains(t, err, "device (esp) is used by multiple things")
}

func TestStorageIsValidUnsupportedFileSystem(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "a",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   nil,
				},
			},
		}},
		BootType: BootTypeEfi,
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				Type:     "ntfs",
			},
		},
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid fileSystemType value (ntfs)")
}

func TestStorageIsValidMountPointWithoutFileSystem(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "a",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   nil,
				},
			},
		}},
		BootType: BootTypeEfi,
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "filesystem with 'mountPoint' must have a 'type'")
}

func TestStorageIsValidMissingFileSystemEntry(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   nil,
					Type:  PartitionTypeESP,
				},
			},
		}},
		BootType: BootTypeEfi,
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "ESP partition (esp) must have 'fat32' or 'vfat' filesystem type")
}

func TestStorageIsValidBadEspFsType(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   nil,
					Type:  PartitionTypeESP,
				},
			},
		}},
		BootType: BootTypeEfi,
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "ext4",
			},
		},
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "ESP partition (esp) must have 'fat32' or 'vfat' filesystem type")
}

func TestStorageIsValidBadBiosBootFsType(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "bios",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   nil,
					Type:  PartitionTypeBiosGrub,
				},
			},
		}},
		BootType: BootTypeEfi,
		FileSystems: []FileSystem{
			{
				DeviceId: "bios",
				Type:     "ext4",
			},
		},
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "BIOS boot partition (bios) must not have a filesystem 'type'")
}

func TestStorageIsValidBadBiosBootStart(t *testing.T) {
	storage := Storage{
		Disks: []Disk{{
			PartitionTableType: PartitionTableTypeGpt,
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "bios",
					Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
					End:   nil,
					Type:  PartitionTypeBiosGrub,
				},
			},
		}},
		BootType: BootTypeLegacy,
	}

	err := storage.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "BIOS boot partition must start at 1 MiB")
}

func TestStorageIsValidBadDeviceId(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(2 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "a",
				Type:     "fat32",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid filesystem item at index 1")
	assert.ErrorContains(t, err, "invalid 'deviceId'")
	assert.ErrorContains(t, err, "device (a) not found")
}

func TestStorageIsValidDuplicatePartitionId(t *testing.T) {
	storage := Storage{
		BootType: BootTypeEfi,
		Disks: []Disk{
			{
				PartitionTableType: PartitionTableTypeGpt,
				MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
					},
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
					},
				},
			},
		},
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				Type:     "ext4",
			},
		},
	}

	err := storage.IsValid()
	assert.ErrorContains(t, err, "invalid disk at index 0")
	assert.ErrorContains(t, err, "invalid partition at index 1")
	assert.ErrorContains(t, err, "duplicate id (a)")
}

func TestStorageIsValid_FilesystemMountPartLabelWithoutLabel_Fail(t *testing.T) {
	storage := Storage{
		BootType: BootTypeEfi,
		Disks: []Disk{
			{
				PartitionTableType: PartitionTableTypeGpt,
				MaxSize:            ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
						Type:  PartitionTypeESP,
					},
				},
			},
		},
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				Type:     FileSystemTypeFat32,
				MountPoint: &MountPoint{
					IdType: MountIdentifierTypePartLabel, // Invalid: no label on partition a
					Path:   "/",
				},
			},
		},
	}

	err := storage.IsValid()
	assert.ErrorContains(t, err, "invalid filesystem item at index 0")
	assert.ErrorContains(t, err, "idType set to 'part-label' but partition (a) has no label set")
}

func TestStorageIsValid_BtrfsSubvolumeMountPartLabelWithoutLabel_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
					Label: "esp",
				},
				{
					Id:    "btrfs",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfs",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								IdType: MountIdentifierTypePartLabel, // Invalid: no label on partition btrfs
								Path:   "/",
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err,
		"idType set to 'part-label' for btrfs subvolume (root) but partition (btrfs) has no label set")
}

func TestStorageIsValidUniqueLabel(t *testing.T) {
	storage := Storage{
		BootType: BootTypeEfi,
		Disks: []Disk{
			{
				PartitionTableType: PartitionTableTypeGpt,
				MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
						Type:  PartitionTypeESP,
						Label: "a",
					},
					{
						Id:    "b",
						Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
						Label: "b",
					},
				},
			},
		},
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				Type:     FileSystemTypeFat32,
				MountPoint: &MountPoint{
					IdType: MountIdentifierTypePartLabel,
					Path:   "/boot/efi",
				},
			},
			{
				DeviceId: "b",
				Type:     FileSystemTypeFat32,
				MountPoint: &MountPoint{
					IdType: MountIdentifierTypePartLabel,
					Path:   "/b",
				},
			},
		},
	}

	err := storage.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_FilesystemMountPointPartLabelWithDuplicateLabeL_Fail(t *testing.T) {
	storage := Storage{
		BootType: BootTypeEfi,
		Disks: []Disk{
			{
				PartitionTableType: PartitionTableTypeGpt,
				MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.MiB)),
				Partitions: []Partition{
					{
						Id:    "a",
						Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
						End:   ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
						Type:  PartitionTypeESP,
						Label: "a",
					},
					{
						Id:    "b",
						Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
						Label: "a",
					},
				},
			},
		},
		FileSystems: []FileSystem{
			{
				DeviceId: "a",
				Type:     FileSystemTypeFat32,
				MountPoint: &MountPoint{
					IdType: MountIdentifierTypePartLabel, // Invalid: duplicate label on partition a
					Path:   "/",
				},
			},
			{
				DeviceId: "b",
				Type:     FileSystemTypeFat32,
				MountPoint: &MountPoint{
					IdType: MountIdentifierTypePartLabel,
					Path:   "/b",
				},
			},
		},
	}

	err := storage.IsValid()
	assert.ErrorContains(t, err, "invalid filesystem item at index 0")
	assert.ErrorContains(t, err, "more than one partition has a label of (a)")
}

func TestStorageIsValid_BtrfsSubvolumeMountPointPartLabelWithDuplicateLabel_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
					Label: "mypartition",
				},
				{
					Id:    "btrfs",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Label: "mypartition",
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfs",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								IdType: MountIdentifierTypePartLabel, // Invalid: duplicate label on partition btrfs
								Path:   "/",
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid filesystem item at index 1")
	assert.ErrorContains(t, err, "more than one partition has a label of (mypartition)")
}

func TestStorageIsValidBothDisksAndResetUuid(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "rootfs",
					Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
		ResetPartitionsUuidsType: ResetPartitionsUuidsTypeAll,
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'resetPartitionsUuidsType' and 'disks'")
}

func TestStorageIsValid_DuplicateMountPointBetweenFilesystems_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "rootfs",
					Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid filesystem item at index 1:\n"+
		"duplicate 'mountPoint.path' (/)")
}

func TestStorageIsValid_DuplicateMountPointBetweenBtrfsSubvolumesSameFilesystem_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfs",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfs",
				Type:     "btrfs",
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
						{
							Path: "var",
							MountPoint: &MountPoint{
								Path: "/home", // Duplicate!
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "duplicate 'mountPoint.path' (/home) in btrfs subvolume (var)")
}

func TestStorageIsValid_DuplicateMountPointBetweenBtrfsSubvolumesDifferentFilesystems_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfs1",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(10 * diskutils.GiB)),
				},
				{
					Id:    "btrfs2",
					Start: ptrutils.PtrTo(DiskSize(10 * diskutils.GiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfs1",
				Type:     "btrfs",
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
			},
			{
				DeviceId: "btrfs2",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "var",
							MountPoint: &MountPoint{
								Path: "/var",
							},
						},
						{
							Path: "home2",
							MountPoint: &MountPoint{
								Path: "/home", // Duplicate with btrfs1!
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "duplicate 'mountPoint.path' (/home) in btrfs subvolume (home2)")
}

func TestStorageIsValid_DuplicateMountPointBetweenBtrfsSubvolumeAndFilesystem_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "boot",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(1024 * diskutils.MiB)),
				},
				{
					Id:    "btrfs",
					Start: ptrutils.PtrTo(DiskSize(1024 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "boot",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/boot",
				},
			},
			{
				DeviceId: "btrfs",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path: "/",
							},
						},
						{
							Path: "boot",
							MountPoint: &MountPoint{
								Path: "/boot", // Duplicate with ext4 filesystem!
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "duplicate 'mountPoint.path' (/boot) in btrfs subvolume (boot)")
}

func TestStorageIsValidFileSystemsWithoutDisks(t *testing.T) {
	value := Storage{
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
		ResetPartitionsUuidsType: ResetPartitionsUuidsTypeAll,
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "cannot specify 'filesystems' without specifying 'disks'")
}

func TestStorageIsValidVerityRoot(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValidVerityUsr(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "usr",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "usrhash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				DeviceId: "usrverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/usr",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "usrverity",
				Name:         "usr",
				DataDeviceId: "usr",
				HashDeviceId: "usrhash",
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValidVerityRootWrongIdType(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:   "rootverity",
				Name: "root",
				DataDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "root",
				},
				HashDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "roothash",
				},
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'verity' with dataDevice/hashDevice and 'disks'")
}

func TestStorageIsValidVerityBothIdTypes(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
				{
					Id: "usr",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "usrhash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:   "rootverity",
				Name: "root",
				DataDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "root",
				},
				HashDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "roothash",
				},
			},
			{
				Id:           "usrverity",
				Name:         "usr",
				DataDeviceId: "usr",
				HashDeviceId: "usrhash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "cannot use both dataDeviceId/hashDeviceId and dataDevice/hashDevice")
}

func TestStorageIsValidVerityRootExistingPartitions(t *testing.T) {
	value := Storage{
		Verity: []Verity{
			{
				Id:   "rootverity",
				Name: "root",
				DataDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "root",
				},
				HashDevice: &IdentifiedPartition{
					IdType: IdentifiedPartitionTypePartLabel,
					Id:     "roothash",
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValidVerityInvalidName(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 0")
	assert.ErrorContains(t, err, "either 'dataDeviceId' or 'dataDevice' must be specified")
}

func TestStorageIsValidVerityDuplicateId(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "root",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 0")
	assert.ErrorContains(t, err, "duplicate id (root)")
}

func TestStorageIsValidVerityBadDataId(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "usr",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 0")
	assert.ErrorContains(t, err, "invalid 'dataDeviceId':\ndevice (usr) not found")
}

func TestStorageIsValidVerityBadHashId(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "usrhash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 0")
	assert.ErrorContains(t, err, "invalid 'hashDeviceId':\ndevice (usrhash) not found")
}

func TestStorageIsValid_VerityFilesystemWrongRootName_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "usr",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must match verity name: 'root' for '/'")
}

func TestStorageIsValid_VerityBtrfsSubvolumesWrongRootName_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path:    "/",
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "usr",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must match verity name: 'root' for '/'")
}

func TestStorageIsValid_VerityBtrfsSubvolumesWrongUsrName_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "usr",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "usrhash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "root",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
			{
				DeviceId: "usrverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "usr",
							MountPoint: &MountPoint{
								Path:    "/usr",
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "usrverity",
				Name:         "root",
				DataDeviceId: "usr",
				HashDeviceId: "usrhash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (usrverity) must match verity name: 'usr' for '/usr'")
}

func TestStorageIsValidVerityHashFileSystem(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
			{
				DeviceId: "roothash",
				Type:     "ext4",
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid filesystem item at index 2")
	assert.ErrorContains(t, err, "invalid 'deviceId'")
	assert.ErrorContains(t, err, "device (roothash) is used by multiple things")
}

func TestStorageIsValid_VerityFileSystemHasIdType_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					IdType:  MountIdentifierTypeUuid,
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "filesystem for verity device (rootverity) may not specify 'mountPoint.idType'")
}

func TestStorageIsValid_VerityBtrfsSubvolumeHasIdType_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path:    "/",
								IdType:  MountIdentifierTypeUuid,
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err,
		"btrfs subvolume (root) for verity device (rootverity) may not specify 'mountPoint.idType'")
}

func TestStorageIsValidVerityFileSystemMissing(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			// Invalid: no filesystem for verity device
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must be set to '/' or '/usr'")
}

func TestStorageIsValidVerityFileSystemMountPointMissing(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				// Invalid: no mount point
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must be set to '/' or '/usr'")
}

func TestStorageIsValid_VerityBtrfsSubvolumeMountPointMissing_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							// Invalid: no mount point
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must be set to '/' or '/usr'")
}

func TestStorageIsValid_VerityFilesystemWithMountPointInvalidPath_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/var", // Invalid: should be "/" for "root" verity device
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must be set to '/' or '/usr'")
}

func TestStorageIsValid_VerityBtrfsSubvolumeWithMountPointInvalidPath_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path:    "/var", // Invalid: should be "/" for "root" verity device
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mount path of verity device (rootverity) must be set to '/' or '/usr'")
}

func TestStorageIsValid_VerityMultipleBtrfsSubvolumeMountPoints_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path:    "/",
								Options: "ro",
							},
						},
						{
							Path: "var",
							MountPoint: &MountPoint{
								Path:    "/var",
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "verity device (rootverity) has multiple mount points, which is not supported")
}

func TestStorageIsValidVerityTwoVerity(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
			{
				Id:           "rootverity2",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "invalid verity item at index 1")
	assert.ErrorContains(t, err, "invalid 'dataDeviceId'")
	assert.ErrorContains(t, err, "device (root) is used by multiple things")
}

func TestStorageIsValid_VerityBtrfsSubvolumeMissingReadonly_Fail(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
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
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "verity device's (rootverity) mount must include the 'ro' mount option")
}

func TestStorageIsValidVerityMissingReadonly(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "verity device's (rootverity) mount must include the 'ro' mount option")
}

func TestStorageIsValid_FilesystemUnexpectedMountPath_LogsWarning(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "rootfs",
					Type:  PartitionTypeVar,
					Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	logMessagesHook := logMessagesHook.AddSubHook()
	defer logMessagesHook.Close()

	err := value.IsValid()

	logMessages := logMessagesHook.ConsumeMessages()

	assert.NoError(t, err)
	assert.Contains(t, logMessages, logger.MemoryLogMessage{
		Message: "Unexpected mount path (/) for partition (rootfs) with type (var). Expected paths: [/var]",
		Level:   logrus.InfoLevel,
	})
}

func TestStorageIsValid_BtrfsSubvolumeUnexpectedMountPath_LogsWarning(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "part1",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeVar,
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "part1",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "vol1",
							MountPoint: &MountPoint{
								Path: "/home", // Unexpected path for 'var' partition type
							},
						},
					},
				},
			},
		},
	}

	logMessagesHook := logMessagesHook.AddSubHook()
	defer logMessagesHook.Close()

	err := value.IsValid()

	logMessages := logMessagesHook.ConsumeMessages()

	assert.NoError(t, err)
	assert.Contains(t, logMessages, logger.MemoryLogMessage{
		Message: "Unexpected mount path (/home) for btrfs subvolume (vol1) on partition (part1) " +
			"with type (var). Expected paths: [/var]",
		Level: logrus.InfoLevel,
	})
}

func TestStorageIsValidBadEspPath(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(4 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "rootfs",
					Start: ptrutils.PtrTo(DiskSize(9 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efj",
				},
			},
			{
				DeviceId: "rootfs",
				Type:     "ext4",
				MountPoint: &MountPoint{
					Path: "/",
				},
			},
		},
	}

	err := value.IsValid()
	assert.ErrorContains(t, err, "ESP partition (esp) must be mounted at /boot/efi")
}

func TestStorageIsValid_BtrfsWithSubvolumes_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfspart",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfspart",
				Type:     "btrfs",
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
						{
							Path: "var",
							MountPoint: &MountPoint{
								Path:    "/var",
								Options: "noatime",
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_BtrfsEmptySubvolumesWithMountPoint_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfspart",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfspart",
				Type:     "btrfs",
				MountPoint: &MountPoint{
					Path: "/",
				},
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{}, // Empty subvolumes, can have MountPoint on filesystem
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_BtrfsNoSubvolumesWithMountPoint_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfspart",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfspart",
				Type:     "btrfs",
				MountPoint: &MountPoint{
					Path: "/",
				},
				// No Btrfs config at all
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_BtrfsSubvolumeWithoutMountPoint_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
				},
				{
					Id:    "btrfspart",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfspart",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path: "/",
							},
						},
						{
							Path: "snapshots", // No mount point - just for snapshots
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_VerityBtrfsSubvolume_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								Path:    "/",
								Options: "ro",
							},
						},
					},
				},
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_VerityBtrfsNoSubvolumes_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			Partitions: []Partition{
				{
					Id: "esp",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 8 * diskutils.MiB,
					},
					Type: PartitionTypeESP,
				},
				{
					Id: "root",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 1 * diskutils.GiB,
					},
				},
				{
					Id: "roothash",
					Size: PartitionSize{
						Type: PartitionSizeTypeExplicit,
						Size: 100 * diskutils.MiB,
					},
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "vfat",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "rootverity",
				Type:     "btrfs",
				MountPoint: &MountPoint{
					Path:    "/",
					Options: "ro",
				},
				// No Btrfs subvolumes - using filesystem-level mount point
			},
		},
		Verity: []Verity{
			{
				Id:           "rootverity",
				Name:         "root",
				DataDeviceId: "root",
				HashDeviceId: "roothash",
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}

func TestStorageIsValid_BtrfsSubvolumeMountPointIdTypePartLabel_Pass(t *testing.T) {
	value := Storage{
		Disks: []Disk{{
			PartitionTableType: "gpt",
			MaxSize:            ptrutils.PtrTo(DiskSize(20 * diskutils.GiB)),
			Partitions: []Partition{
				{
					Id:    "esp",
					Start: ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
					End:   ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Type:  PartitionTypeESP,
					Label: "esp",
				},
				{
					Id:    "btrfspart",
					Start: ptrutils.PtrTo(DiskSize(512 * diskutils.MiB)),
					Label: "btrfsroot",
				},
			},
		}},
		BootType: "efi",
		FileSystems: []FileSystem{
			{
				DeviceId: "esp",
				Type:     "fat32",
				MountPoint: &MountPoint{
					Path: "/boot/efi",
				},
			},
			{
				DeviceId: "btrfspart",
				Type:     "btrfs",
				Btrfs: &BtrfsConfig{
					Subvolumes: []BtrfsSubvolume{
						{
							Path: "root",
							MountPoint: &MountPoint{
								IdType: MountIdentifierTypePartLabel,
								Path:   "/",
							},
						},
						{
							Path: "home",
							MountPoint: &MountPoint{
								IdType: MountIdentifierTypePartLabel,
								Path:   "/home",
							},
						},
					},
				},
			},
		},
	}

	err := value.IsValid()
	assert.NoError(t, err)
}
