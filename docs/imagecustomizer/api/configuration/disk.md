---
parent: Configuration
ancestor: Image Customizer
---

# disk type

Specifies the properties of a disk, including its partitions.

Added in v0.3.

## partitionTableType [string]

Specifies how the partition tables are laid out.

Supported options:

- `gpt`: Use the GUID Partition Table (GPT) format.

## maxSize [string]

The size of the disk.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

This value is optional if the last partition on the disk has an explicit size.

```yaml
disks:
- partitionTableType: gpt
  maxSize: 4G
  partitions:
  - id: esp
    type: esp
    size: 8M

  - id: rootfs
    size: grow
```

Added in v0.3.

## partitions [[partition](./partition.md)[]]

The partitions to provision on the disk.

Partitions must be specified in order, from the start of the disk to the end.

Added in v0.3.
