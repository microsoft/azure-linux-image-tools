---
title: Command line
parent: API
grand_parent: Image Creator
nav_order: 3
---

# Image Creator command line

## --help

Displays the tool's quick help.

## --build-dir=DIRECTORY-PATH

Required.

The directory where the tool will place its temporary files.

## --tools-file=FILE-PATH

Required.

Specifies the path to a tools file in `.tar.gz` format.

This file should contain the TDNF tar package (or an equivalent), which is used to manage package
dependencies and facilitate installation workflows.

## --output-image-file=FILE-PATH

Required, unless
[output.image.path](../../imagecustomizer/api/configuration/outputImage.md#path-string) is
provided in the configuration file. If both `output.image.path` and
`--output-image-file` are provided, then the `--output-image-file` value
is used.

The file path to write the created image to.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

## --output-image-format=FORMAT

Required, unless
[output.image.format](../../imagecustomizer/api/configuration/outputImage.md#format-string) is
provided in the configuration file. If both `output.image.format` and
`--output-image-format` are provided, then the `--output-image-format`
value is used.

The format type of the image.

Options: vhd, vhd-fixed, vhdx, qcow2, raw.

The vhd-fixed option outputs a fixed size VHD image. This is the required format for
VMs in Azure.

## --config-file=FILE-PATH

Required.

The file path of the YAML (or JSON) configuration file that specifies the configuration of the image
to be created.

For documentation on the supported configuration options, see:
[Image Creator configuration](./configuration/configuration.md)

## --rpm-source=PATH

Required.

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

  The repo file will only be used during image creation and will not be added to
  the image.
  If you want to add the repo file to the image, then use use
  [additionalFiles](../../imagecustomizer/api/configuration/os.md#additionalfiles-additionalfile) to
  place the repo file under the `/etc/yum.repos.d` directory.

This option can be specified multiple times.

See, [Building custom packages](../../imagecustomizer/reference/building-packages.md) for a guide on
how to build your own packages for Azure Linux.

See, [Cloning an RPM repo](../../imagecustomizer/reference/clone-rpm-repo.md) for how to clone or download
RPMs from a existing RPM repo (such as packages.microsoft.com). Using a cloned repo with
`--rpm-source` can help your builds avoid dependencies on external resources.

## --log-level=LEVEL

Default: `info`

The verbosity of logs the tool outputs.

Higher levels of logging may be useful for debugging what the tool is doing.

The levels from lowest to highest level of verbosity are: `panic`, `fatal`, `error`,
`warn`, `info`, `debug`, and `trace`.

## --package-snapshot-time

Limits package selection to those published before the specified timestamp.

This flag enables snapshot-based package filtering during installation or update,
ensuring only packages available at that point in time are considered.

Supports:

- A date in `YYYY-MM-DD` format (interpreted as UTC midnight)
- A full RFC 3339 timestamp (e.g., `2024-05-20T23:59:59Z`)

You may enable this feature by adding `package-snapshot-time` to the
[previewfeatures](../../imagecustomizer/api/configuration/config.md#previewfeatures-string) API.
