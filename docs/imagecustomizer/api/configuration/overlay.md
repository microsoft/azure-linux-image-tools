---
parent: Configuration
ancestor: Image Customizer
---

# overlay type

Specifies the configuration for overlay filesystem.

Example:

```yaml
os:
  overlays:
  - mountPoint: /media
    lowerDirs:
    - /media
    - /home
    upperDir: /overlays/media/upper
    workDir: /overlays/media/work
```

## `mountPoint` [string]

The directory where the combined view of the `upperDir` and `lowerDir` will be
mounted. This is the location where users will see the merged contents of the
overlay filesystem. It is common for the `mountPoint` to be the same as the
`lowerDir`. But this is not required.

Example: `/etc`

Added in v0.6.

## `lowerDirs` [string[]]

A list of directories that act as the read-only layers in the overlay filesystem. They
contain the base files and directories which will be overlaid by the `upperDir`.

Example:

```yaml
lowerDirs: 
- /etc
```

Added in v0.6.

## `upperDir` [string]

Required.

This directory is the writable layer of the overlay filesystem. Any
modifications, such as file additions, deletions, or changes, are made in the
`upperDir`. These changes are what make the overlay filesystem appear different
from the lowerDir alone.

Example: `/var/overlays/etc/upper`

Added in v0.6.

## `workDir` [string]

Required.

This directory is used for preparing files before they are merged
into the `upperDir`. It needs to be on the same filesystem as the `upperDir` and
is used for temporary storage by the overlay filesystem to ensure atomic
operations. The `workDir` should not be directly accessed by users.

Example: `/var/overlays/etc/work`

Added in v0.6.

## `isInitrdOverlay` [bool]

A boolean flag indicating that this overlay should be provisioned by the initramfs
before the pivot to the root filesystem.
This should be set to `true` for overlays targeting fundamental system directories such
as `/etc`.

Setting this value to `true` will result to the following changes to the overlay:

- The lower, upper, and work directories' paths will have the `/sysroot` prefix added to
  them, since that is the path of the root filesystem before the pivot.

- The mount options `x-initrd.mount,x-systemd.wanted-by=initrd-fs.target` will be added
  to the overlay.

Default value: `false`.

Example:

```yaml
storage:
  disks:
  bootType: efi
  - partitionTableType: gpt
    partitions:
    - id: esp
      type: esp
      size: 8M

    - id: rootfs
      label: rootfs
      size: 2G

    - id: var
      size: 2G

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077

  - deviceId: rootfs
    type: ext4
    mountPoint: /

  - deviceId: var
    type: ext4
    mountPoint:
      path: /var
      options: defaults,x-initrd.mount

os:
  bootloader:
    resetType: hard-reset

  overlays:
  - mountPoint: /etc
    lowerDirs:
    - /etc
    upperDir: /var/overlays/etc/upper
    workDir: /var/overlays/etc/work
    isInitrdOverlay: true
    mountDependencies:
    - /var
```

Added in v0.7.

## `mountDependencies` [string[]]

Specifies a list of directories that must be mounted before this overlay. Each
directory in the list should be mounted and available before the overlay
filesystem is mounted.

This is an optional argument.

Example:

```yaml
mountDependencies: 
- /var
```

**Important**: If any directory specified in `mountDependencies` needs to be
available during the initrd phase, you must ensure that this directory's mount
configuration in the `filesystems` section includes the `x-initrd.mount` option.

For example:

```yaml
filesystems:
- deviceId: var
  type: ext4
  mountPoint:
    path: /var
    options: defaults,x-initrd.mount
```

Added in v0.6.

## `mountOptions` [string]

A string of additional mount options that can be applied to the overlay mount.
Multiple options should be separated by commas.

This is an optional argument.

Example: `noatime,nodiratime`

Added in v0.6.
