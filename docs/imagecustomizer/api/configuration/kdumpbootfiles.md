---
parent: Configuration
ancestor: Image Customizer
---

# kdumpBootFiles [string]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `crash-dump` in the
[previewFeatures](./config.md#previewfeatures-string) API or
via the [--preview-feature](../cli/cli.md#--preview-featurefeature) flag.

Specifies the configuration for how to handle kdump boot files in Live OS
formats (iso and pxe).

The kdump boot files include:

- a crashdump initramfs image named `initramfs-<kernel-version>kdump.img`.
- a kernel named `vmlinuz-<kernel-version>` - where its version matches that of
  the `initramfs-<kernel-version>kdump.img`.

By default, the Image Customizer tool removes the `/boot` folder from the full
OS image. This is to avoid file duplication after all the contents have already
been copied unto the iso media directly (or to the pxe artifacts). However, if
kdump is configured, the kdump kernel and initramfs files must be preserved
under the `/boot` folder.

Supported options:

- `none`: kdump files are not preserved in the full OS image under the `/boot`
  folder.
- `keep`: kdump files are preserved in the full OS image under the `/boot`
  folder.

The default value is `none`.

Added in v0.16.
