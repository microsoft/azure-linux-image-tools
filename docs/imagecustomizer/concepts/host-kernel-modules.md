---
parent: Concepts
title: Host Kernel Modules
nav_order: 8
---

# Host Kernel Modules

Image Customizer mounts the input disk image and its partitions on the host so
that it can modify them offline. These mounts happen in kernel space, so the
host kernel must provide a few modules for Image Customizer to work.

This applies whether you run Image Customizer as a
[binary](../quick-start/quick-start-binary.md) or via the
[container image](../quick-start/quick-start.md): the container shares the
host's kernel, so any module needed at runtime must be available on the host.

## Core functionality

- `loop`: Attaches disk image files as block devices via `losetup`. Used on
  every run.
- `ext4`: Mounts ext4 partitions. Used whenever the image has ext4 (nearly all
  Azure Linux images).
- `vfat`: Mounts FAT32 EFI System Partitions. Used whenever the image has UEFI
  boot.
- `btrfs`: Mounts btrfs partitions and subvolumes. Used only when the image
  uses btrfs.
- `xfs`: Mounts XFS partitions. Used only when the image uses XFS.

## Verity images

Required when the image has [verity](./verity/verity.md) features enabled.

- `dm_mod`: Device-mapper core. Used when verity features are enabled.
- `dm_verity`: Device-mapper verity target. Used when verity features are
  enabled.

## ISO and Live OS

Required only when working with [ISO](./iso.md) or [Live OS](./liveos.md)
images.

- `squashfs`: Mounts SquashFS filesystems. Used only when building Live OS /
  ISO images.
- `iso9660`: Mounts ISO 9660 filesystems. Used only when re-customizing an
  existing ISO image.
