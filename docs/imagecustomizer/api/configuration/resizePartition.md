---
parent: Configuration
ancestor: Image Customizer
---

# resizePartition type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `resize-disk` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Contains options for resizing a partition.

Added in v1.8.

## ref [string]

The partition to target.

Supported values:

- `last`: The last partition on the disk (according to the disk's physical
   layout, not the partition table ordering).

Added in v1.8.

## freeSpace [string]

The minimum amount of free space the partition's filesystem should have.
If the filesystem already has sufficient free space, no changes are made.
Otherwise, the partition is expanded.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`),
or TiB (`T`). Must be a multiple of 1 MiB.

```yaml
previewFeatures:
- resize-disk

storage:
  resizeDisk:
    partitions:
    - ref: last
      freeSpace: 50G
```

Added in v1.8.

## size [string]

The new size of the parition

Shrinking the disk size is not supported.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`),
or TiB (`T`). Must be a multiple of 1 MiB.

Example:

```yaml
previewFeatures:
- resize-disk

storage:
  resizeDisk:
    partitions:
    - ref: last
      size: 50G
```

Added in v1.8.
