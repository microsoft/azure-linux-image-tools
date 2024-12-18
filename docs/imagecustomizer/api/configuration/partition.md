# partition type

## id [string]

Required.

The ID of the partition.
This is used to correlate Partition objects with [filesystem](./filesystem.md)
objects.

## label [string]

The label to assign to the partition.

## start [uint64]

Required.

The start location (inclusive) of the partition.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

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

## size [uint64]

The size of the partition.

Supported formats:

- `<NUM>(K|M|G|T)`: An explicit size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`).

- `grow`: Fill up the remainder of the disk. Must be the last partition.

Must be a multiple of 1 MiB.

## type [string]

Specifies options for the partition.

Supported options:

- `esp`: The UEFI System Partition (ESP).
  The partition must have a `fileSystemType` of `fat32` or `vfat`.

- `bios-grub`: Specifies this partition is the BIOS boot partition.
  This is required for GPT disks that wish to be bootable using legacy BIOS mode.

  This partition must start at block 1.

  This flag is only supported on GPT formatted disks.

  For further details, see: https://en.wikipedia.org/wiki/BIOS_boot_partition
