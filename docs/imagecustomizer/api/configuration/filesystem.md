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

## Customizing existing EROFS filesystems

Existing ext4 and Btrfs behavior is unchanged by this support.
Finalization requires a clean unmount; a busy mount fails customization instead
of lazily detaching a filesystem that might still be in use.

Existing block-backed EROFS `/usr` partitions are copied, with metadata, into
writable build-directory staging and repacked after customization. This requires
`erofs-utils` and sufficient build-directory capacity for the unpacked tree and
the repacked image. Repacking preserves the filesystem UUID and label, emits
uncompressed single-device EROFS with 4096-byte blocks, and fails before writing
the partition if the new image does not fit. EROFS is not a supported `type`
for creating or repartitioning filesystems through this API.
The staging host must preserve extended attributes, including their absence.
If host SELinux labeling adds attributes to originally unlabeled paths,
staging verification fails instead of accepting a relabeled image. Use a
compatible build environment; do not disable metadata verification.
Image discovery also searches EROFS for the ACL layout marker
`share/ic/etc/fstab`, allowing conversion of existing sealed `/usr` images to
COSI without changing their filesystem format.

ACL customization preserves existing SELinux labels rather than relabeling the
whole `/usr` tree. A newly created or unlabeled `/etc` upper directory inherits
the baseline's label before folding. A host-assigned label on a newly created
directory is not treated as a pre-existing image label; an already labeled
upper directory remains authoritative. Temporary resolver replacement
preserves the original extended attributes.

Verity-backed partitions remain read-only unless `storage.reinitializeVerity:
all` is explicitly requested with the corresponding preview feature. EROFS
customization supports separate hash partitions covering the whole data
partition, not inline verity. Regenerated root hashes require new signatures;
the caller must sign the finalized output rather than reuse an old signature.
