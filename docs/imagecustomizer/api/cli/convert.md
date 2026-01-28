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

## --image-file=FILE-PATH

Required.

The path to the image to convert.

Supported input formats: `vhd`, `vhd-fixed`, `vhdx`, `qcow2`, `raw`, and `iso`.

## --output-image-file=FILE-PATH

Required.

The file path to write the converted image to.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

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

## --cosi-compression-level=LEVEL

Optional.

Zstd compression level for COSI output (valid range: 1-22, default: 9).

Higher values provide better compression but take longer to compress.
Lower values compress faster but result in larger files.

This option is only applicable when `--output-image-format` is set to `cosi`
or `baremetal-image`.

When using custom compression levels, the `cosi-compression` preview feature
must be enabled in the configuration file (if applicable).

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
