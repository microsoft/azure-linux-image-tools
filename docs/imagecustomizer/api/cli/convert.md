---
title: convert
parent: Command line
ancestor: Image Customizer
nav_order: 3
---

# convert subcommand

Converts an image from one format to another without performing any customization
or file injection. This is useful for simple format conversion tasks, especially
when converting to COSI or bare-metal-image formats.

## Usage

```bash
imagecustomizer convert \
  --build-dir /tmp/build \
  --image-file input.vhdx \
  --output-image-file output.cosi \
  --output-image-format cosi
```

Added in v1.2.

## --build-dir=DIRECTORY-PATH

Required.

The temporary workspace directory where the tool will place its working files.

Added in v1.2.

## --image-file=FILE-PATH

Required.

The path to the image to convert.

Supported input formats: `vhd`, `vhd-fixed`, `vhdx`, `qcow2`, `raw`, and `iso`.

Added in v1.2.

## --output-image-file=FILE-PATH

Required.

The file path to write the converted image to.

Added in v1.2.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

Added in v1.2.

## --output-image-format=FORMAT

Required.

The format type of the output image.

Supported formats:
- `vhd`: Dynamic VHD format
- `vhd-fixed`: Fixed-size VHD format (required for Azure VMs)
- `vhdx`: Hyper-V VHDX format
- `qcow2`: QEMU QCOW2 format
- `raw`: Raw disk image format
- `cosi`: Compressed image format with metadata
- `baremetal-image`: COSI format with VHD footer for bare-metal deployments

Added in v1.2.

## --cosi-compression-level=LEVEL

Optional. Default: `9`

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `cosi-compression` in the
[previewFeatures](../configuration/config.md#previewfeatures-string) API.

The zstd compression level (1-22) for COSI partition images.

Higher compression levels produce smaller files but take significantly longer to
compress. Decompression speed is largely unaffected by the compression level.

Added in v1.2.

## Examples

### Convert VHDX to COSI

```bash
imagecustomizer convert \
  --build-dir ./build \
  --image-file base-image.vhdx \
  --output-image-file converted-image.cosi \
  --output-image-format cosi
```

### Convert QCOW2 to VHD Fixed (for Azure)

```bash
imagecustomizer convert \
  --build-dir ./build \
  --image-file vm-image.qcow2 \
  --output-image-file azure-image.vhd \
  --output-image-format vhd-fixed
```

### Convert with Custom COSI Compression

```bash
imagecustomizer convert \
  --build-dir ./build \
  --image-file large-image.raw \
  --output-image-file compressed-image.cosi \
  --output-image-format cosi \
  --cosi-compression-level 15
```
