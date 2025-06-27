---
parent: Configuration
---

# crashDump type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `crash-dump` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Defines the configuration for the crash dump behavior.

Added in v0.16.

## keepKdumpBootFiles [bool]

This property is applicable only to Live OS formats (iso and pxe).

If set to true, the Image Customizer tool will not delete any kdump files found
under the boot folder on the full OS image. The kdump files include:

- a crashdump initramfs image named `initramfs-<kernel-version>kdump.img`.
- a kernel named  `vmlinuz-<kernel-version>` - where its version matches that of
  the `initramfs-<kernel-version>kdump.img`.

The default is `false`.

Note that by default, the Image Customizer tool removes the `/boot` folder from
the full OS image. This is because all of its contents have already been copied
unto the iso media directly (or to the pxe artifacts).

Added in v0.16.
