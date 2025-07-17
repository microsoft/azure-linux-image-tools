---
parent: Configuration
ancestor: Image Customizer
---

# outputImage type

Specifies the configuration for the output image.

Example:

```yaml
image:
  path: ./out/image.vhdx
  format: vhdx
```

## path [string]

Required, unless
[--output-image-file](../cli.md#--output-image-filefile-path) is provided
on the command line. If both `--output-image-file` and `output.image.path`
are provided, then the value of `--output-image-file` is used.

The file path to write the final customized image to.

Added in v0.13.

## format [string]

Required, unless
[--output-image-format](../cli.md#--output-image-formatformat) is provided
on the command line. If both `--output-image-format` and
`output.image.format` are provided, then the value of
`--output-image-format` is used.

The image format of the final customized image.

Options: vhd, vhd-fixed, vhdx, qcow2, raw, iso, and [cosi](../cosi.md).

The vhd-fixed option outputs a fixed size VHD image. This is the required format for
VMs in Azure.

When the output image format is set to iso, the generated image is a LiveOS
iso image. For more details on this format, see:
[Image Customizer ISO Support](../../concepts/iso.md).

Added in v0.13.
