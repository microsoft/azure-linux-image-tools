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

If verity is enabled in the base image, then:

- If the partitions are recustomized using the
  [disks](../api/configuration/storage.md#disks-disk) API, then the existing verity
  settings are thrown away.
  New verity settings can be configured with the
  [verity](../api/configuration/verity.md) API.

- Otherwise, the existing verity settings are reapplied to the image after OS
  customization.

  This feature is in preview and may be subject to breaking changes.
  You may enable this feature by adding `reinitialize-verity` to the
  [previewfeatures](./configuration/config.md#previewfeatures-string) API.

Added in v0.3.

## --output-image-file=FILE-PATH

Required, unless
[output.image.path](./configuration/outputImage.md#path-string) is
provided in the configuration file. If both `output.image.path` and
`--output-image-file` are provided, then the `--output-image-file` value
is used.

The file path to write the final customized image to.

If the `--output-format` is set to `pxe`, this can be set to either:
- the name of a directory where the customized PXE artifacts will be placed.
- the name of a a `.tar.gz` where the customized PXE artifacts will be archived.

Added in v0.3.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

Added in v0.15.

## --output-image-format=FORMAT

Required, unless
[output.image.format](./configuration/outputImage.md#format-string) is
provided in the configuration file. If both `output.image.format` and
`--output-image-format` are provided, then the `--output-image-format`
value is used.

The image format of the final customized image.

Options: vhd, vhd-fixed, vhdx, qcow2, raw, iso, pxe, and [cosi](./cosi.md).

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

  GPG signature checking is disabled for local directories.
  If you wish to enable GPG signature checking, then use a repo file instead and set the
  `gpgkey` field within the repo file.

- `*.repo` file path: A path to a RPM repo definition file.

  The file name extension must be `.repo`.

  If the repo file's `baseurl` or `gpgkey` fields contain a `file://` URL, then the
  host's directories pointed to by the URL will be bind mounted into the chroot
  environment and the URL will be replaced with the chroot equivalent URL.

  GPG signature checking is enabled by default.
  If you wish to disable GPG checking, then set both `gpgcheck` and `repo_gpgcheck` to
  `0` in the repo file.

  The repo file will only be used during image customization and will not be added to
  the image.
  If you want to add the repo file to the image, then use use
  [additionalFiles](../api/configuration/os.md#additionalfiles-additionalfile) to place
  the repo file under the `/etc/yum.repos.d` directory.

This option can be specified multiple times.

RPM sources are specified in the order or priority from lowest to highest.
If `--disable-base-image-rpm-repos` is not specified, then the in-built RPM repos are
given the lowest priority.

See, [Building custom packages](../reference/building-packages.md) for a guide on how to
build your own packages for Azure Linux.

See, [Cloning an RPM repo](../reference/clone-rpm-repo.md) for how to clone or download
RPMs from a existing RPM repo (such as packages.microsoft.com). Using a cloned repo with
`--rpm-source` can help your builds avoid dependencies on external resources.

Added in v0.3.

## --disable-base-image-rpm-repos

Disable the base image's installed RPM repos as a source of RPMs during package
installation.

Added in v0.3.

## --log-level=LEVEL

Default: `info`

The verbosity of logs the tool outputs.

Higher levels of logging may be useful for debugging what the tool is doing.

The levels from lowest to highest level of verbosity are: `panic`, `fatal`, `error`,
`warn`, `info`, `debug`, and `trace`.

Added in v0.3.

## inject-files

Injects files into a disk image using an injection configuration.

See [inject-files subcommand](./cli/inject-files.md) for full documentation.

## --package-snapshot-time

Limits package selection to those published before the specified timestamp.

This flag enables snapshot-based package filtering during installation or update,
ensuring only packages available at that point in time are considered.

Supports:
- A date in `YYYY-MM-DD` format (interpreted as UTC midnight)
- A full RFC 3339 timestamp (e.g., `2024-05-20T23:59:59Z`)

You may enable this feature by adding `package-snapshot-time` to the [previewfeatures]
(./configuration/config.md#previewfeatures-string) API.

Added in v0.15.
