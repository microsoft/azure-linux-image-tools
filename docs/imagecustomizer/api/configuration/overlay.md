---
parent: Configuration
---

# overlay type

Specifies the configuration for overlay filesystem.

Overlays Configuration Example:

```yaml
storage:
  disks:
  bootType: efi
  - partitionTableType: gpt
    maxSize: 4G
    partitions:
    - id: esp
      type: esp
      start: 1M
      end: 9M
    - id: boot
      start: 9M
      end: 108M
    - id: rootfs
      label: rootfs
      start: 108M
      end: 2G
    - id: var
      start: 2G

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077
  - deviceId: boot
    type: ext4
    mountPoint:
      path: /boot
  - deviceId: rootfs
    type: ext4
    mountPoint:
      path: /
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

## `lowerDirs` [string[]]

These directories act as the read-only layers in the overlay filesystem. They
contain the base files and directories which will be overlaid by the `upperDir`.
Multiple lower directories can be specified by providing a list of paths, which
will be joined using a colon (`:`) as a separator.

Example:

```yaml
lowerDirs: 
- /etc
```

## `upperDir` [string]

This directory is the writable layer of the overlay filesystem. Any
modifications, such as file additions, deletions, or changes, are made in the
upperDir. These changes are what make the overlay filesystem appear different
from the lowerDir alone.
  
Example: `/var/overlays/etc/upper`

## `workDir` [string]

This is a required directory used for preparing files before they are merged
into the upperDir. It needs to be on the same filesystem as the upperDir and
is used for temporary storage by the overlay filesystem to ensure atomic
operations. The workDir is not directly accessible to users. 
  
Example: `/var/overlays/etc/work`

## `isInitrdOverlay` [bool]

A boolean flag indicating whether this overlay is part of the root filesystem.
If set to `true`, specific adjustments will be made, such as prefixing certain
paths with `/sysroot`, and the overlay will be added to the fstab file with the
`x-initrd.mount` option to ensure it is available during the initrd phase.

This is an optional argument.

Example: `False`

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

## `mountOptions` [string]

A string of additional mount options that can be applied to the overlay mount.
Multiple options should be separated by commas.

This is an optional argument.

Example: `noatime,nodiratime`
