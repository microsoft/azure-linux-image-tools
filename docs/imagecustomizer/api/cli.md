---
title: Command line
parent: API
nav_order: 1
---

# Image Customizer command line

## --help

Displays the tool's quick help.

## --build-dir=DIRECTORY-PATH

Required.

The directory where the tool will place its temporary files.

Added in v0.3.

## --image-file=FILE-PATH

Required, unless [input.image.path](./configuration/inputImage.md#path-string) is
provided in the configuration file. If both `input.image.path` and
`--image-file` are provided, then the `--image-file` value is used.

The base image file to customize.

This file is typically one of the standard Azure Linux core images.
But it can also be an Azure Linux image that has been customized.

Supported image file formats: vhd, vhdx, qcow2, and raw.

Added in v0.3.

## --output-image-file=FILE-PATH

Required, unless
[output.image.path](./configuration/outputImage.md#path-string) is
provided in the configuration file. If both `output.image.path` and
`--output-image-file` are provided, then the `--output-image-file` value
is used.

The file path to write the final customized image to.

Added in v0.3.

## --output-image-format=FORMAT

Required, unless
[output.image.format](./configuration/outputImage.md#format-string) is
provided in the configuration file. If both `output.image.format` and
`--output-image-format` are provided, then the `--output-image-format`
value is used.

The image format of the final customized image.

Options: vhd, vhd-fixed, vhdx, qcow2, raw, iso, and [cosi](./cosi.md).

The vhd-fixed option outputs a fixed size VHD image. This is the required format for
VMs in Azure.

When the output image format is set to iso, the generated image is a LiveOS
iso image. For more details on this format, see:
[Image Customizer ISO Support](../concepts/iso.md).

## --config-file=FILE-PATH

Required.

The file path of the YAML (or JSON) configuration file that specifies how to customize
the image.

For documentation on the supported configuration options, see:
[Image Customizer configuration](./configuration.md)

Added in v0.3.

## --rpm-source=PATH

A resource that provides RPM files to be used during package installation.

Can be one of:

- Directory path: A path to a directory containing RPM files.

  The RPMs may either be in the directory itself or any subdirectories.

- `*.repo` file path: A path to a RPM repo definition file.

  The file name extension must be `.repo`.

  Note: This file is not installed in the image during customization.
  If that is also needed, then use `AdditionalFiles` to place the repo file within
  the image.

This option can be specified multiple times.

RPM sources are specified in the order or priority from lowest to highest.
If `--disable-base-image-rpm-repos` is not specified, then the in-built RPM repos are
given the lowest priority.

See, [Building custom packages](../how-to/building-packages.md) for a guide on how to
build your own packages for Azure Linux.

See, [Cloning an RPM repo](../how-to/clone-rpm-repo.md) for how to clone or download
RPMs from a existing RPM repo (such as packages.microsoft.com). Using a cloned repo with
`--rpm-source` can help your builds avoid dependencies on external resources.

Added in v0.3.

## --disable-base-image-rpm-repos

Disable the base image's installed RPM repos as a source of RPMs during package
installation.

Added in v0.3.

## --output-pxe-artifacts-dir

Create a folder containing the artifacts to be used for PXE booting.

For an overview of Image Customizer support for PXE, see the
[PXE support page](../concepts/pxe.md).

Added in v0.8.

## --log-level=LEVEL

Default: `info`

The verbosity of logs the tool outputs.

Higher levels of logging may be useful for debugging what the tool is doing.

The levels from lowest to highest level of verbosity are: `panic`, `fatal`, `error`,
`warn`, `info`, `debug`, and `trace`.

Added in v0.3.

## inject-files (Subcommand)

This subcommand takes a base image and a config file (typically auto-generated
from the [output.artifacts](./configuration/outputArtifacts.md) API) and injects
files (like signed bootchain artifacts) back into the image at specified locations.

The image formats supported are `vhd`, `vhdx`, `qcow2`, and `raw`. The output
image will be written to the same path as the input image (in-place modification).

The injection config must have the `inject-files`
[previewFeatures](./configuration/config.md#previewfeatures-string) enabled.

See [`injectFilesConfig`](./configuration/injectFilesConfig.md) for config format.

### Required Arguments

- `--config-file=FILE-PATH`

  Path to the file injection configuration.

- `--image-file=FILE-PATH`

  Path to the image file to modify.

- `--build-dir=DIRECTORY-PATH`

  Temporary workspace directory.

Example:

```bash
imagecustomizer \
  inject-files \
  --config-file /path/to/inject-files.yaml \
  --image-file /path/to/input-image.qcow2 \
  --build-dir /path/to/build-dir
```
