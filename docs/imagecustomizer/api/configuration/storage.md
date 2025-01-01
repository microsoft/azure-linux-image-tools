# storage type

## bootType [string]

Specifies the boot system that the image supports.

Supported options:

- `legacy`: Support booting from BIOS firmware.

  When this option is specified, the partition layout must contain a partition with the
  `bios-grub` flag.

- `efi`: Support booting from UEFI firmware.

  When this option is specified, the partition layout must contain a partition with the
  `esp` flag.

## disks [[disk](./disk.md)[]]

Contains the options for provisioning disks and their partitions.

## verity [[verity](./verity.md)[]]

Configure verity block devices.

## filesystems [[filesystem](./filesystem.md)[]]

Specifies the mount options of the partitions.

## resetPartitionsUuidsType [string]

Specifies that the partition UUIDs and filesystem UUIDs should be reset.

Value is optional.

This value cannot be specified if [storage](./storage.md) is specified (since
customizing the partition layout resets all the UUIDs anyway).

If this value is specified, then [os.bootloader.resetType](./bootloader.md#resettype-string)
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
