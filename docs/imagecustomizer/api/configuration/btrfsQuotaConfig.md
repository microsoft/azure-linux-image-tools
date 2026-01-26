---
parent: Configuration
ancestor: Image Customizer
---

# btrfsQuotaConfig type

Defines quota settings for a BTRFS subvolume.

Example:

```yaml
previewFeatures:
- btrfs

storage:
  filesystems:
  - deviceId: btrfs
    type: btrfs
    btrfs:
      subvolumes:
      - path: root
        mountPoint:
          path: /
        quota:
          referencedLimit: 10G
          exclusiveLimit: 8G
      - path: home
        mountPoint:
          path: /home
        quota:
          referencedLimit: 50G
      - path: var
        mountPoint:
          path: /var
        quota:
          referencedLimit: 20G
```

Added in v1.2.

## referencedLimit [string]

Optional.

Maximum total space the subvolume can use ("referenced" limit),
including data shared with other subvolumes via snapshots or reflinks.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`).

Added in v1.2.

## exclusiveLimit [string]

Optional.

Maximum unshared space the subvolume can use ("exclusive" limit). Only counts data unique to this subvolume,
excluding data shared via snapshots or reflinks.

This option is useful if snapshots are created at runtime.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`).

Added in v1.2.
