---
parent: Configuration
ancestor: Image Customizer
---

# resizeDisk type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `resize-disk` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Contains options for resizing a disk and partitions without changing the
partition ordering or partition metadata (e.g. partition UUID, filesystem UUID,
etc.).

If only the last partition's size is changed, then the disk is expanded and the
last partition is grown without changing any other partitions or the disk's
header. If any other partition's size is changed, then a new disk layout is
created with all previous gaps between partitions removed while maintaining the
default partition alignment of 1 MiB.

Added in v1.8.

## diskSize [string]

The new size of the disk. The last partition is expanded to fill the new size of
the disk.

Shrinking the disk size is not supported.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`),
or TiB (`T`). Must be a multiple of 1 MiB.

Example:

```yaml
previewFeatures:
- resize-disk

storage:
  resizeDisk:
    diskSize: 50G
```

Added in v1.8.

## partitions [[resizePartition](./resizePartition.md)[]]

Specify the new sizes of the disks's partitions. The disk is automatically
resized to contain the new partition sizes.

Example:

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
