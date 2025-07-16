---
parent: Configuration
ancestor: Image Customizer
---

# partition type

Added in v0.3.

## id [string]

Required.

The ID of the partition.
This is used to correlate Partition objects with [filesystem](./filesystem.md)
objects.

Added in v0.3.

## label [string]

The label to assign to the partition.

Added in v0.3.

## start [uint64]

The start location (inclusive) of the partition.

If this value is not specified, then a partition starts at the end of the previous
partition.
The first partition will by default start at 1 MiB to leave space for the partition
table.

Partitions must be specified in order, from the start of the disk to the end.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

Added in v0.3.

## end [uint64]

The end location (exclusive) of the partition.

The `end` and `size` fields cannot be specified at the same time.

Either the `size` or `end` field is required for all partitions except for the last
partition.
When both the `size` and `end` fields are omitted or when the `size` field is set to the
value `grow`, the last partition will fill the remainder of the disk based on the disk's
[maxSize](./disk.md#maxsize-uint64) field.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

Example:

```yaml
disks:
- partitionTableType: gpt
  partitions:
  - id: esp
    type: esp
    start: 1M
    end: 8M

  - id: rootfs
    start: 8M
    end: 4096M
```

Added in v0.3.

## size [uint64]

The size of the partition.

Supported formats:

- `<NUM>(K|M|G|T)`: An explicit size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`).

- `grow`: Fill up the remainder of the disk. Must be the last partition.

Must be a multiple of 1 MiB.

Example:

```yaml
disks:
- partitionTableType: gpt
  partitions:
  - id: esp
    type: esp
    size: 8M

  - id: rootfs
    size: 4G
```

Added in v0.3.

## type [string]

Specifies the type of partition.

This field must be specified for a UEFI System Partition (`esp`) or a BIOS boot
partition (`bios-grub`) but is otherwise optional.

Supported options:

- `esp`: The UEFI System Partition (ESP).
  The partition must have a `fileSystemType` of `fat32` or `vfat`.

- `bios-grub`: Specifies this partition is the BIOS boot partition.
  This is required for GPT disks that wish to be bootable using legacy BIOS mode.

  This partition must start at block 1.

  This flag is only supported on GPT formatted disks.

  For further details, see: [Wikipedia's BIOS boot partition article](https://en.wikipedia.org/wiki/BIOS_boot_partition)

- `home`: A `/home` partition.

- `linux-generic`: A generic Linux partition.

  This is the default value.

- `root`: The `/` partition.

- `root-verity`: The verity hash partition for `/`.

- `srv`: The `/srv` partition.

- `swap`: A swap partition.

- `tmp`: The `/var/tmp` partition.

- `usr`: The `/usr` partition.

- `usr-verity`: The verity hash partition for `/usr`.

  Note: Image Customizer does not yet support `/usr` verity partitions.

- `var`: The `/var` partition.

- `xbootldr`: The `/boot` partition.

- A partition type UUID string.

  A list of well-known UUID values can be found in:
  
  - [Wikipedia's GUID Partition Table article](https://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_type_GUIDs)
  - [The Discoverable Partitions Specification (DPS)](https://uapi-group.org/specifications/specs/discoverable_partitions_specification/#defined-partition-type-uuids)

  For example: `c12a7328-f81f-11d2-ba4b-00a0c93ec93b`

Example:

```yaml
disks:
- partitionTableType: gpt
  partitions:
  - id: esp
    type: esp
    size: 8M

  - id: rootfs
    type: root
    size: 4G
```

Added in v0.3.
