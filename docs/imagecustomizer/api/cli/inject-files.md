---
title: inject-files
parent: Command line
nav_order: 1
---

# inject-files subcommand

This subcommand takes a base image and a config file (typically auto-generated
from the [output.artifacts](../configuration/outputArtifacts.md) API) and injects
files (like signed bootchain artifacts) back into the image at specified locations.

The output image will be written to the same path as the input image, unless
`--output-image-file` and `--output-image-format` are specified.

The injection config must have the `inject-files`
[previewFeatures](../configuration/config.md#previewfeatures-string) enabled.

See [`injectFilesConfig`](../configuration/injectFilesConfig.md) for config format.

## --config-file=FILE-PATH

Required.

The path to the file injection configuration, typically auto-generated from
the `output.artifacts` section of the image customization config.

See: [injectFilesConfig](../configuration/injectFilesConfig.md)

## --image-file=FILE-PATH

Required.

The path to the base image to inject files into.

Supported image formats: `vhd`, `vhdx`, `qcow2`, and `raw`.

## --build-dir=DIRECTORY-PATH

Required.

The temporary workspace directory where the tool will place its working files.

## --output-image-file=FILE-PATH

Optional.

The file path to write the modified image to. If not specified, the image
is modified at `--image-file`.

## --output-image-format=FORMAT

Optional.

The image format of the final image written to `--output-image-file`.

Options: `vhd`, `vhd-fixed`, `vhdx`, `qcow2`, `raw`, `iso`, and `cosi`.

If this option is not provided, the format of the input image is preserved.
