---
parent: Configuration
ancestor: Image Customizer
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
