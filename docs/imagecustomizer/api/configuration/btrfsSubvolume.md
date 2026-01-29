---
parent: Configuration
ancestor: Image Customizer
---

# btrfsSubvolume type

Specifies a subvolume within a BTRFS file system.

Example:

```yaml
previewFeatures:
- btrfs

storage:
  bootType: efi

  disks:
  - partitionTableType: gpt
    maxSize: 20G
    partitions:
    - id: esp
      type: esp
      size: 512M

    - id: btrfs
      size: grow

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077

  - deviceId: btrfs
    type: btrfs
    btrfs:
      subvolumes:
      - path: root
        mountPoint:
          path: /
          options: compress=zstd:1,noatime
      - path: home
        mountPoint:
          path: /home
          options: compress=zstd:1,noatime
      - path: root/var/lib/postgresql
        mountPoint:
          path: /var/lib/postgresql
          options: nodatacow
```

Added in v1.2.

## path [string]

Required.

The path to the subvolume within the top-level subvolume (subvolid=5), relative to its root.

If parents of the path are not other subvolumes, they will be created as regular directories.

Added in v1.2.

## mountPoint [[mountPoint](./mountpoint.md)]

Optional settings for where and how to mount the subvolume.

Added in v1.2.

## quota [[btrfsQuotaConfig](./btrfsQuotaConfig.md)]

Optional.

Quota settings for this subvolume. When specified, BTRFS quotas (qgroups) are automatically enabled on the filesystem.

Added in v1.2.
