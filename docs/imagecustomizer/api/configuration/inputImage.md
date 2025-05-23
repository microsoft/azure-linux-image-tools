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

If verity is enabled in the base image, then:

- If the partitions are recustomized using the
  [disks](storage.md#disks-disk) API, then the existing verity
  settings are thrown away.
  New verity settings can be configured with the
  [verity](verity.md) API.

- Otherwise, the existing verity settings are reapplied to the image after OS
  customization.

  This feature is in preview and may be subject to breaking changes.
  You may enable this feature by adding `reinitialize-verity` to the
  [previewfeatures](config.md#previewfeatures-string) API.

Added in v0.13.
