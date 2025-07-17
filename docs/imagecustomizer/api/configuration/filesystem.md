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

- `ext4`
- `fat32` (alias for `vfat`)
- `vfat` (will select either FAT12, FAT16, or FAT32 based on the size of the partition)
- `xfs`

Added in v0.3.

## mountPoint [[mountPoint](./mountpoint.md)]

Optional settings for where and how to mount the filesystem.

Added in v0.3.
