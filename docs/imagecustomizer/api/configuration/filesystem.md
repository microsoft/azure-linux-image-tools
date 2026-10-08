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

## Customizing existing sealed filesystems

Existing ext4 filesystems using the native `read-only` feature are temporarily
unsealed for writable customization with checksum-aware e2fsprogs operations,
then checked and resealed before verity is regenerated. Their partition geometry
is preserved during COSI export.
Finalization requires a clean unmount; a busy mount fails customization instead
of lazily detaching a filesystem that might still be in use.

Existing block-backed EROFS `/usr` partitions are copied, with metadata, into
writable build-directory staging and repacked after customization. This requires
`erofs-utils` and sufficient build-directory capacity for the unpacked tree and
the repacked image. Repacking preserves the filesystem UUID and label, emits
uncompressed single-device EROFS with 4096-byte blocks, and fails before writing
the partition if the new image does not fit. EROFS is not a supported `type`
for creating or repartitioning filesystems through this API.

Verity-backed partitions remain read-only unless `storage.reinitializeVerity:
all` is explicitly requested with the corresponding preview feature. EROFS
customization supports separate hash partitions covering the whole data
partition, not inline verity. Regenerated root hashes require new signatures;
the caller must sign the finalized output rather than reuse an old signature.
