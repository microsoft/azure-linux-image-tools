
# storage [[storage](#storage-type)]

Contains the options for provisioning disks, partitions, and file systems.

While Disks is a list, only 1 disk is supported at the moment. Support for
multiple disks may (or may not) be added in the future.

```yaml
storage:
  bootType: efi

  disks:
  - partitionTableType: gpt
    maxSize: 4096M
    partitions:
    - id: esp
      type: esp
      start: 1M
      end: 9M

    - id: rootfs
      start: 9M

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077

  - deviceId: rootfs
    type: ext4
    mountPoint:
      path: /

os:
  bootloader:
    resetType: hard-reset
```

- [storage \[storage\]](#storage-storage)
  - [bootType \[string\]](#boottype-string)
  - [resetPartitionsUuidsType \[string\]](#resetpartitionsuuidstype-string)
  - [disks \[disk\[\]\]](#disks-disk)
    - [disk type](#disk-type)
      - [partitionTableType \[string\]](#partitiontabletype-string)
      - [maxSize \[uint64\]](#maxsize-uint64)
      - [partitions \[partition\[\]\]](#partitions-partition)
        - [partition type](#partition-type)
          - [id \[string\]](#id-string)
          - [label \[string\]](#label-string)
          - [start \[uint64\]](#start-uint64)
          - [end \[uint64\]](#end-uint64)
          - [size \[uint64\]](#size-uint64)
          - [type \[string\]](#type-string)
      - [filesystems \[filesystem\[\]\]](#filesystems-filesystem)
        - [filesystem type](#filesystem-type)
          - [deviceId \[string\]](#deviceid-string)
          - [type \[string\]](#type-string-1)
          - [mountPoint \[mountPoint\]](#mountpoint-mountpoint)
          - [mountPoint type](#mountpoint-type)
  - [verity \[verity\[\]\]](#verity-verity)
    - [verity type](#verity-type)
      - [id \[string\]](#id-string-1)
      - [name \[string\]](#name-string)
      - [dataDeviceId \[string\]](#datadeviceid-string)
      - [hashDeviceId \[string\]](#hashdeviceid-string)
      - [corruptionOption \[string\]](#corruptionoption-string)

## bootType [string]

Specifies the boot system that the image supports.

Supported options:

- `legacy`: Support booting from BIOS firmware.

  When this option is specified, the partition layout must contain a partition with the
  `bios-grub` flag.

- `efi`: Support booting from UEFI firmware.

  When this option is specified, the partition layout must contain a partition with the
  `esp` flag.

## resetPartitionsUuidsType [string]

Specifies that the partition UUIDs and filesystem UUIDs should be reset.

Value is optional.

This value cannot be specified if [storage](#storage-storage) is specified (since
customizing the partition layout resets all the UUIDs anyway).

If this value is specified, then [os.resetBootLoaderType](#resetbootloadertype-string)
must also be specified.

Supported options:

- `reset-all`: Resets the partition UUIDs and filesystem UUIDs for all the partitions.

Example:

```yaml
storage:
  resetPartitionsUuidsType: reset-all

os:
  bootloader:
    resetType: hard-reset
```

## disks [[disk](#disk-type)[]]

Contains the options for provisioning disks and their partitions.

### disk type

Specifies the properties of a disk, including its partitions.

#### partitionTableType [string]

Specifies how the partition tables are laid out.

Supported options:

- `gpt`: Use the GUID Partition Table (GPT) format.

#### maxSize [uint64]

The size of the disk.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`),
or TiB
(`T`).

Must be a multiple of 1 MiB.

#### partitions [[partition](#partition-type)[]]

The partitions to provision on the disk.

##### partition type

<div id="partition-id"></div>

###### id [string]

Required.

The ID of the partition.
This is used to correlate Partition objects with [filesystem](#filesystem-type)
objects.

###### label [string]

The label to assign to the partition.

###### start [uint64]

Required.

The start location (inclusive) of the partition.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

###### end [uint64]

The end location (exclusive) of the partition.

The `end` and `size` fields cannot be specified at the same time.

Either the `size` or `end` field is required for all partitions except for the last
partition.
When both the `size` and `end` fields are omitted or when the `size` field is set to the
value `grow`, the last partition will fill the remainder of the disk based on the disk's
[maxSize](#maxsize-uint64) field.

Supported format: `<NUM>(K|M|G|T)`: A size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB
(`T`).

Must be a multiple of 1 MiB.

###### size [uint64]

The size of the partition.

Supported formats:

- `<NUM>(K|M|G|T)`: An explicit size in KiB (`K`), MiB (`M`), GiB (`G`), or TiB (`T`).

- `grow`: Fill up the remainder of the disk. Must be the last partition.

Must be a multiple of 1 MiB.

<div id="partition-type-string"></div>

###### type [string]

Specifies options for the partition.

Supported options:

- `esp`: The UEFI System Partition (ESP).
  The partition must have a `fileSystemType` of `fat32` or `vfat`.

- `bios-grub`: Specifies this partition is the BIOS boot partition.
  This is required for GPT disks that wish to be bootable using legacy BIOS mode.

  This partition must start at block 1.

  This flag is only supported on GPT formatted disks.

  For further details, see: https://en.wikipedia.org/wiki/BIOS_boot_partition

#### filesystems [[filesystem](#filesystem-type)[]]

Specifies the mount options of the partitions.

##### filesystem type

Specifies the mount options for a partition.

###### deviceId [string]

Required.

The ID of the [partition](#partition-type) or [verity](#verity-type) object.

###### type [string]

Required.

The filesystem type of the partition.

Supported options:

- `ext4`
- `fat32` (alias for `vfat`)
- `vfat` (will select either FAT12, FAT16, or FAT32 based on the size of the partition)
- `xfs`

###### mountPoint [[mountPoint](#mountpoint-type)]

Optional settings for where and how to mount the filesystem.

###### mountPoint type

You can configure `mountPoint` in one of two ways:

1. **Structured Format**: Use `idType`, `options`, and `path` fields for a more detailed configuration.

   ```yaml
   mountPoint:
     path: /boot/efi
     options: umask=0077
     idType: part-uuid
   ```

2. **Shorthand Path Format**: Provide the mount path directly as a string when only `path` is required.

   ```yaml
   mountPoint: /boot/efi
   ```

   In this shorthand format, only the `path` is specified, and default values will be applied to any optional fields.

######## idType [string]

Default: `part-uuid`

The partition ID type that should be used to recognize the partition on the disk.

Supported options:

- `uuid`: The filesystem's partition UUID.

- `part-uuid`: The partition UUID specified in the partition table.

- `part-label`: The partition label specified in the partition table.

######## options [string]

The additional options used when mounting the file system.

These options are in the same format as
[mount](https://man7.org/linux/man-pages/man8/mount.8.html)'s
`-o` option (or the `fs_mntops` field of the
[fstab](https://man7.org/linux/man-pages/man5/fstab.5.html) file).

<div id="mountpoint-path"></div>

######## path [string]

Required.

The absolute path of where the partition should be mounted.

The mounts will be sorted to ensure that parent directories are mounted before child
directories.
For example, `/boot` will be mounted before `/boot/efi`.

## verity [[verity](#verity-type)[]]

Configure verity block devices.

### verity type

Specifies the configuration for dm-verity integrity verification.

Note: Currently only root partition (`/`) is supported. Support for other partitions
(e.g. `/usr`) may be added in the future.

Note: The [filesystem](#filesystem-type) item pointing to this verity device, must
include the `ro` option in the [mountPoint.options](#options-string).

There are multiple ways to configure a verity enabled image. For
recommendations, see [Verity Image Recommendations](./verity.md).

<div id="verity-id"></div>

#### id [string]

Required.

The ID of the verity object.
This is used to correlate verity objects with [filesystem](#filesystem-type)
objects.

<div id="verity-name"></div>

#### name [string]

Required.

The name of the device mapper block device.

The value must be:

- `root` for root partition (i.e. `/`)

#### dataDeviceId [string]

The ID of the [partition](#partition-type) to use as the verity data partition.

#### hashDeviceId [string]

The ID of the [partition](#partition-type) to use as the verity hash partition.

#### corruptionOption [string]

Optional.

Specifies how a mismatch between the hash and the data partition is handled.

Supported values:

- `io-error`: Fails the I/O operation with an I/O error.
- `ignore`: Ignores the corruption and continues operation.
- `panic`: Causes the system to panic (print errors) and then try restarting.
- `restart`: Attempts to restart the system.

Default value: `io-error`.
