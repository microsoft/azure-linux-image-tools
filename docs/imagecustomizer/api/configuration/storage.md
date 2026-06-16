---
parent: Configuration
ancestor: Image Customizer
---

# storage type

Contains the options for provisioning disks, partitions, and file systems.

Example:

```yaml
storage:
  bootType: efi

  disks:
  - partitionTableType: gpt
    maxSize: 4G
    partitions:
    - id: esp
      type: esp
      size: 8M

    - id: rootfs
      size: grow

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077

  - deviceId: rootfs
    type: ext4
    mountPoint: /

os:
  bootloader:
    resetType: hard-reset
```

Added in v0.3.

## bootType [string]

Specifies the boot system that the image supports.

Supported options:

- `legacy`: Support booting from BIOS firmware.

  The `grub2-install` command must be installed in the target image.

  Install the following packages (using the `os.packages.install` API):

  - Fedora, Azure Linux 4.0: `grub2-tools`, `grub-pc-modules`
  - Azure Linux 2.0, 3.0: `grub2`, `grub2-pc`

  For Azure Linux 2.0 and 3.0, if the `grub2-install` command is not available in the
  target OS, then the `grub2-install` command from the container (or build host) is
  used instead. However, this behavior is deprecated and will be removed in a future
  version.

  When this option is specified, the partition layout must contain a partition with the
  `bios-grub` flag.

- `efi`: Support booting from UEFI firmware.

  When this option is specified, the partition layout must contain a partition with the
  `esp` flag.

Example:

```yaml
storage:
  disks:
  - partitionTableType: gpt
    partitions:
    - id: boot
      type: bios-grub
      size: 8M

    - id: rootfs
      size: 4G

  bootType: legacy

  filesystems:
  - deviceId: rootfs
    type: ext4
    mountPoint: /

os:
  bootloader:
    resetType: hard-reset
```

Added in v0.3.

## disks [[disk](./disk.md)[]]

Contains the options for provisioning disks and their partitions.

Note: While disks is a list, only 1 disk is supported at the moment.
Support for multiple disks may (or may not) be added in the future.

Added in v0.3.

## verity [[verity](./verity.md)[]]

Configure verity block devices.

Added in v0.7.

## filesystems [[filesystem](./filesystem.md)[]]

Specifies the mount options of the partitions.

Added in v0.3.
(Renamed from `fileSystems` to `filesystems` in v0.7.)

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

Added in v0.7.

## reinitializeVerity [string]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `reinitialize-verity` in the
[previewFeatures](./config.md#previewfeatures-string) API.

When the base image contains verity partitions, controls whether or not the verity
partitions can be modified.

Supported options:

- `none`: During OS customization, all verity data partitions are mounted as read-only.
  The verity hash partitions are left unchanged.

- `all`: During OS customization, all verity data partitions are mounted as read-write.
  The verity hash partitions will be regenerated.

Added in v0.17.
