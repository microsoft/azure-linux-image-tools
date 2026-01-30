---
title: create
parent: Command line
ancestor: Image Customizer
nav_order: 1
---

# create subcommand

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `create` in the [previewFeatures](../configuration/config.md#previewfeatures-string) API.

This subcommand creates a new Azure Linux image from scratch using a configuration file and RPM sources.
Unlike the default [customize](customize.md) subcommand which modifies an existing image, `create` builds an
entirely new image.

Added in v1.2.

## --build-dir=DIRECTORY-PATH

Required.

The directory where the tool will place its temporary files.

Added in v1.2.

## --output-image-file=FILE-PATH

Required, unless
[output.image.path](../configuration/outputImage.md#path-string) is
provided in the configuration file. If both `output.image.path` and
`--output-image-file` are provided, then the `--output-image-file` value
is used.

The file path to write the created image to.

Added in v1.2.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

Added in v1.2.

## --output-image-format=FORMAT

Required, unless
[output.image.format](../configuration/outputImage.md#format-string) is
provided in the configuration file. If both `output.image.format` and
`--output-image-format` are provided, then the `--output-image-format`
value is used.

The image format of the created image.

Options: vhd, vhd-fixed, vhdx, qcow2, raw.

The vhd-fixed option outputs a fixed size VHD image. This is the required format for
VMs in Azure.

Added in v1.2.

## --config-file=FILE-PATH

Required.

The file path of the YAML (or JSON) configuration file that specifies how to create
the image.

For documentation on the supported configuration options, see:
[Image Customizer configuration](../configuration/configuration.md)

Added in v1.2.

## --rpm-source=PATH

Required.

A resource that provides RPM files to be used during package installation.

Can be one of:

- Directory path: A path to a directory containing RPM files.

  The RPMs may either be in the directory itself or any subdirectories.

  GPG signature checking is disabled for local directories.
  If you wish to enable GPG signature checking, then use a repo file instead and set the
  `gpgkey` field within the repo file.

- `*.repo` file path: A path to an RPM repo definition file.

  The file name extension must be `.repo`.

  If the repo file's `baseurl` or `gpgkey` fields contain a `file://` URL, then the
  host's directories pointed to by the URL will be bind mounted into the chroot
  environment and the URL will be replaced with the chroot equivalent URL.

  GPG signature checking is enabled by default.
  If you wish to disable GPG checking, then set both `gpgcheck` and `repo_gpgcheck` to
  `0` in the repo file.

  The repo file will only be used during image creation and will not be added to
  the image.
  If you want to add the repo file to the image, then use
  [additionalFiles](../configuration/os.md#additionalfiles-additionalfile) to place
  the repo file under the `/etc/yum.repos.d` directory.

This option can be specified multiple times.

RPM sources are specified in the order or priority from lowest to highest.

See, [Building custom packages](../../reference/building-packages.md) for a guide on how to
build your own packages for Azure Linux.

See, [Cloning an RPM repo](../../reference/clone-rpm-repo.md) for how to clone or download
RPMs from an existing RPM repo (such as packages.microsoft.com). Using a cloned repo with
`--rpm-source` can help your builds avoid dependencies on external resources.

Added in v1.2.

## --tools-file=PATH

Required.

Specifies the path to a tools file in `.tar.gz` format.

This file should contain the TDNF tar package (or an equivalent), which is used to manage package dependencies and facilitate installation workflows.

Added in v1.2.

## --distro=DISTRO

Optional. Default: `azurelinux`

Specifies the distribution of the image to be built.

Supported values: `azurelinux`, `fedora`.

Added in v1.2.

## --distro-version=VERSION

Optional.

Specifies the distribution version of the image to be built.

Supported versions:

- For `azurelinux`: `2.0`, `3.0`
- For `fedora`: `42`

Added in v1.2.

## --package-snapshot-time=TIMESTAMP

Optional.

Limits package selection to those published before the specified timestamp.

This flag enables snapshot-based package filtering during installation or update,
ensuring only packages available at that point in time are considered.

Supports:

- A date in `YYYY-MM-DD` format (interpreted as UTC midnight)
- A full RFC 3339 timestamp (e.g. `2024-05-20T23:59:59Z`)

You may enable this feature by adding `package-snapshot-time` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v1.2.
