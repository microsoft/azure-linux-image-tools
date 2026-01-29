---
parent: Configuration
ancestor: Image Customizer
---

# btrfsConfig type

BTRFS-specific filesystem configuration options.

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
```

Added in v1.2.

## subvolumes [[btrfsSubvolume](./btrfsSubvolume.md)]

Optional.

A list of subvolumes to create within the BTRFS file system.

Subvolumes are independently mountable POSIX file trees within a BTRFS file system.

Added in v1.2.
