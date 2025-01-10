---
parent: Configuration
---

# disk type

Specifies the properties of a disk, including its partitions.

## partitionTableType [string]

Specifies how the partition tables are laid out.

Supported options:

- `gpt`: Use the GUID Partition Table (GPT) format.

## maxSize [uint64]

The size of the disk.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

## partitions [[partition](./partition.md)[]]

The partitions to provision on the disk.
