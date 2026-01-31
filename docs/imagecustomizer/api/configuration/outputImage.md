---
parent: Configuration
ancestor: Image Customizer
---

# outputImage type

Specifies the configuration for the output image.

Example:

```yaml
output::
  image:
    path: ./out/image.vhdx
    format: vhdx
```

## path [string]

Required, unless
[--output-image-file](../cli/customize.md#--output-image-filefile-path) is provided
on the command line. If both `--output-image-file` and `output.image.path`
are provided, then the value of `--output-image-file` is used.

The file path to write the final customized image to.

Added in v0.13.

## format [string]

Required, unless
[--output-image-format](../cli/customize.md#--output-image-formatformat) is provided
on the command line. If both `--output-image-format` and
`output.image.format` are provided, then the value of
`--output-image-format` is used.

The image format of the final customized image.

Supported options:

- `vhd`: Outputs a dynamic VHD image.

- `vhd-fixed`: Outputs a fixed size VHD image. This is the required format for VMs in Azure.

- `vhdx`: Outputs a VHDX image.

- `qcow2`: Outputs a QCOW2 image.

- `raw`: Outputs a raw disk image.

- `iso`: Outputs a LiveOS ISO image. For more details, see: [Image Customizer ISO Support](../../concepts/iso.md).

- `pxe-dir`: Outputs a PXE boot directory.

- `pxe-tar`: Outputs a tarball containing a PXE boot directory.

- `cosi`: Outputs a [Composable Operating System Image](../cosi.md).

- `baremetal-image`: Outputs a [Composable Operating System Image](../cosi.md) for baremetal deployments.

Added in v0.13.

## cosi [[cosiConfig](./cosiConfig.md)]

Optional.

Specifies the configuration options for the COSI (Composable Operating System
Image) output format.

This field is only applicable when `format` is set to `cosi`.

Example:

```yaml
output:
  image:
    path: ./out/image.cosi
    format: cosi
    cosi:
      compression:
        level: 22
previewFeatures:
- cosi-compression
```

Added in v1.2.

