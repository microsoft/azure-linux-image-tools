---
parent: Configuration
---

# inputImage type

Specifies the configuration for the input image.

Example:

```yaml
image:
  path: ./base.vhdx
```

## path [string]

Required, unless [--image-file](../cli.md#--image-filefile-path) is
provided on the command line. If both `--image-file` and
`input.image.path` are provided, then the value of `--image-file` is used.

The base image file to customize.

This file is typically one of the standard Azure Linux core images.
But it can also be an Azure Linux image that has been customized.

Supported image file formats: vhd, vhdx, qcow2, and raw.

Added in v0.13.0.
