---
parent: Configuration
---

# iso type

Specifies the configuration for the generated ISO image when the `--output-format`
is set to `iso`.

Example:

```yaml
iso:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt

  kernelCommandLine:
    extraCommandLine:
    - rd.info

  initramfsType: bootstrap
```

See also: [ISO Support](../../concepts/iso.md)

## keepKdumpBootFiles [bool]

If set to true, the Image Customizer tool will not delete any kdump files found
under the boot folder on the full OS image. The kdump files include:

- a crashdump initramfs image named `initramfs-<kernel-version>kdump.img`.
- a kernel named  `vmlinuz-<kernel-version>` - where its version matches that of
  the `initramfs-<kernel-version>kdump.img`.

The default is `false`.

Note that by default, the Image Customizer tool removes the `/boot` folder from
the full OS image. This is because all of its contents have already been copied
unto the root of the PXE artifacts folder.

Added in v0.16.

## kernelCommandLine [[kernelCommandLine](./kernelcommandline.md)]

Specifies extra kernel command line options.

Added in v0.3.

## additionalFiles [[additionalFile](./additionalfile.md)[]]

Adds files to the ISO.

Added in v0.7.

## initramfsType [string]

Specifies the initramfs type to generate and include in the ISO image.

Supported options:

- `bootstrap`: Creates a minimal Dracut-based initramfs image that later
  transitions to the full OS. The full OS is packaged in a separate image
  and is included on the media. This option allows the generated ISO to boot
  on hardware that has memory restrictions on the initramfs image size.
- `full-os`: Creates a full OS initramfs image.

The default value for `initramfsType` is `bootstrap`.

Note that SELinux cannot be enabled if `initramfsType` is set to `full-os`.

Example:

```yaml
iso:
  initramfsType: full-os
```

Added in v0.15.
