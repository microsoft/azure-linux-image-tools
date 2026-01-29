---
parent: Configuration
ancestor: Image Customizer
---

# filesystem type

Specifies the mount options for a partition.

Added in v0.3.

## deviceId [string]

Required.

The ID of the [partition](./partition.md) or [verity](./verity.md) object.

Added in v0.3.

## type [string]

Required.

The filesystem type of the partition.

Supported options:

- `btrfs`:
  This is a preview feature.
  Its API and behavior is subject to change.
  You must enable this feature by specifying `btrfs` in the [previewFeatures](./config.md#previewfeatures-string) API.

- `ext4`
- `fat32`: This is an alias for `vfat`

- `vfat`: This will select either FAT12, FAT16, or FAT32 based on the size of the partition.

- `xfs`

Added in v0.3.

## mountPoint [[mountPoint](./mountpoint.md)]

Optional settings for where and how to mount the filesystem.

This cannot be set when [.btrfs.subvolumes](./btrfsConfig.md#subvolumes-btrfssubvolume) is configured.
Use mount points on individual BTRFS subvolumes instead.

Added in v0.3.

## btrfs [[btrfsConfig](./btrfsConfig.md)]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `btrfs` in the [previewFeatures](./config.md#previewfeatures-string) API.

Optional.

BTRFS-specific configuration options.

This can only be set when [.type](./filesystem.md#type-string) is `btrfs`.

Added in v1.2.
