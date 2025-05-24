---
title: inject-files
parent: Command line
nav_order: 1
---

# inject-files subcommand

This is a preview feature.
Its API and behavior is subject to change.
You must enabled this feature by specifying `inject-files` in the
[previewFeatures](../configuration/injectFilesConfig.md#previewfeatures-string) API.

This subcommand takes a base image and a config file (typically auto-generated
from the [output.artifacts](../configuration/outputArtifacts.md) API) and injects
files (like signed bootchain artifacts) back into the image at specified locations.

The output image will be written to the same path as the input image, unless
`--output-image-file` and `--output-image-format` are specified.

See [`injectFilesConfig`](../configuration/injectFilesConfig.md) for config format.

Added in v0.14.

## --config-file=FILE-PATH

Required.

The path to the file injection configuration, typically auto-generated from
the `output.artifacts` section of the image customization config.

See: [injectFilesConfig](../configuration/injectFilesConfig.md)

Added in v0.14.

## --image-file=FILE-PATH

Required.

The path to the base image to inject files into.

Supported image formats: `vhd`, `vhdx`, `qcow2`, and `raw`.

Added in v0.14.

## --build-dir=DIRECTORY-PATH

Required.

The temporary workspace directory where the tool will place its working files.

Added in v0.14.

## --output-image-file=FILE-PATH

Optional.

The file path to write the modified image to. If not specified, the image
is modified at `--image-file`.

Added in v0.14.

## --output-image-format=FORMAT

Optional.

The image format of the final image written to `--output-image-file`.

Options: `vhd`, `vhd-fixed`, `vhdx`, `qcow2`, `raw`, `iso`, and `cosi`.

If this option is not provided, the format of the input image is preserved.

Added in v0.14.
