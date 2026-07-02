---
title: customize
parent: Command line
ancestor: Image Customizer
nav_order: 3
---

# customize subcommand

This subcommand takes a base OS image and modifies it according to the provided config
file.

If no subcommand is specified, this subcommand is the default.

Added in v0.13. Prior to v0.13, there were no subcommands and `customize` was the only
operation supported.

## --build-dir=DIRECTORY-PATH

Required.

The directory where the tool will place its temporary files.

Added in v0.3.

## --image-file=FILE-PATH

The base image file to customize.

An input image must either be provided in the configuration file (e.g.
[input.image.path](../configuration/inputImage.md#path-string)) or on the command line.
If both a command-line input image and a configuration input image are specified, then
the command line's input image overrides the config file's input image.

This file is typically an official image file of one of the supported distros.
But it can also be an image that has already been customized.

Supported image file formats: vhd, vhdx, qcow2, and raw.

Note: Image Customizer will reject VHD files created by `qemu-img` unless the
`-o force_size=on` option was passed. Without this option, `qemu-img` will
likely change the size of the disk (to a non-multiple of 1 MiB), which can cause
problems when trying to upload the disk to Azure.

If verity is enabled in the base image, then:

- If the partitions are recustomized using the
  [disks](../configuration/storage.md#disks-disk) API, then the existing verity
  settings are thrown away.
  New verity settings can be configured with the
  [verity](../configuration/verity.md) API.

- Otherwise, the existing verity settings are reapplied to the image after OS
  customization, according to the
  [.storage.reinitializeVerity](../configuration/storage.md#reinitializeverity-string)
  setting.

  This feature is in preview and may be subject to breaking changes.
  You may enable this feature by adding `reinitialize-verity` to the
  [previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v0.3.

## --image

Specifies the location where the base image can be downloaded from.

Supported formats:

- `azurelinux:<VARIANT>:<VERSION>`

  Where:

  - `<VARIANT>`: The variant of the Azure Linux image.
  
  - `<VERSION>`: The version of the Azure Linux image.
  
  See [azureLinuxImage](../configuration/azurelinuximage.md) for more details.

- `oci:<URI>`

  Where:

  - `<URI>`: The URI of the OCI artifact containing the image.

  See [ociImage](../configuration/ociimage.md) for more details.

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

When using this option, you must also specify [--image-cache-dir](#--image-cache-dir).

Added in v1.1.

## --output-image-file=FILE-PATH

Required, unless
[output.image.path](../configuration/outputImage.md#path-string) is
provided in the configuration file. If both `output.image.path` and
`--output-image-file` are provided, then the `--output-image-file` value
is used.

The file path to write the final customized image to.

Added in v0.3.

## --output-path=FILE-PATH

An alias to `--output-image-file`.

Added in v0.15.

## --output-image-format=FORMAT

Required, unless
[output.image.format](../configuration/outputImage.md#format-string) is
provided in the configuration file. If both `output.image.format` and
`--output-image-format` are provided, then the `--output-image-format`
value is used.

The image format of the final customized image.

Supported image formats: `vhd`, `vhd-fixed`, `vhdx`, `qcow2`, `raw`, `iso`, `pxe-dir`, `pxe-tar`, `cosi`,
and `baremetal-image`.

See [output.image.format](../configuration/outputImage.md#format-string) for detailed descriptions
of each format.

## --cosi-compression-level=LEVEL

Optional. Default: `9`

If both
[output.image.cosi.compression.level](../configuration/cosiCompression.md#level-int) and
`--cosi-compression-level` are provided, then the `--cosi-compression-level`
value is used.

The zstd compression level (1-22) for COSI partition images.

Higher compression levels produce smaller files but take significantly longer to
compress. Decompression speed is largely unaffected by the compression level.

Added in v1.2.

## --output-selinux-policy-path=DIRECTORY-PATH

Optional.

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-selinux-policy` in the
[previewFeatures](../configuration/config.md#previewfeatures-string) API.

If both
[output.selinuxPolicyPath](../configuration/output.md#selinuxpolicypath-string) and
`--output-selinux-policy-path` are provided, then the `--output-selinux-policy-path`
value is used.

The directory path to output the SELinux policy files extracted from the customized
image. The SELinux policy type is determined by reading the `SELINUXTYPE` value from
`/etc/selinux/config` in the image (e.g. `targeted`), and the corresponding directory
(e.g. `/etc/selinux/targeted`) will be extracted and copied to this location.

If not specified, SELinux policy extraction is disabled.

Added in v1.1.

## --config-file=FILE-PATH

Required.

The file path of the YAML (or JSON) configuration file that specifies how to customize
the image.

For documentation on the supported configuration options, see:
[Image Customizer configuration](../configuration/configuration.md)

Added in v0.3.

## --rpm-source=PATH

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
  environment and the URL will be replaced with the chroot equivalent URL. A
  `file+rel://` URL is handled similarly but the host path is relative to the repo
  file's parent directory. A `file+chroot://` URL refers to a file within the chroot
  itself. If the tools chroot is being used (e.g. when using the [create](create.md)
  subcommand), then this path is within the tools chroot instead of the OS chroot.

  If you use a repo file pointing to local directory containing RPM files, then you must
  call `createrepo_c` (or `createrepo`) on the directory before using it as a repo:

  ```bash
  createrepo_c --compatibility --update <rpms-directory>
  ```

  GPG signature checking is enabled by default.
  If you wish to disable GPG checking, then set both `gpgcheck` and `repo_gpgcheck` to
  `0` in the repo file.

  The repo file will only be used during image customization and will not be added to
  the image.
  If you want to add the repo file to the image, then use
  [additionalFiles](../configuration/os.md#additionalfiles-additionalfile) to place
  the repo file under the `/etc/yum.repos.d` directory.

This option can be specified multiple times.

If you need to prioritize one repo over another, then use a `*.repo` file and specify
the `priority` field.

See, [Building custom packages](../../reference/building-packages.md) for a guide on how to
build your own packages for Azure Linux.

See, [Cloning an RPM repo](../../reference/clone-rpm-repo.md) for how to clone or download
RPMs from an existing RPM repo (such as packages.microsoft.com). Using a cloned repo with
`--rpm-source` can help your builds avoid dependencies on external resources.

Added in v0.3.

## --disable-base-image-rpm-repos

Disable the base image's installed RPM repos as a source of RPMs during package
installation.

Added in v0.3.

## --package-snapshot-time

Limits package selection to those published before the specified timestamp.

This flag enables snapshot-based package filtering during installation or update,
ensuring only packages available at that point in time are considered.

Supports:

- A date in `YYYY-MM-DD` format (interpreted as UTC midnight)
- A full RFC 3339 timestamp (e.g. `2024-05-20T23:59:59Z`)

You may enable this feature by adding `package-snapshot-time` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v0.15.

## --image-cache-dir

A directory path that can be used to cache downloaded image files so that they can be
reused in subsequent runs.

This option is used in conjunction with the
[oci](../configuration/inputImage.md#oci-ociimage) API.

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v1.1.

## --tools-dir=DIRECTORY-PATH

Optional.

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `tools-dir` in the
[previewFeatures](../configuration/config.md#previewfeatures-string) API.

Specifies the path to a directory that provides an external package manager (tdnf/dnf).
Required when performing package operations on images that do not include a package
manager in the base image.

When provided, the directory is copied into a separate chroot environment. The image is
mounted inside that chroot at `/_imageroot`, and tdnf is invoked with
`--installroot=/_imageroot`. Images that already include tdnf (e.g. standard Azure
Linux images) do not need this flag.

For instructions on how to create this directory, see:
[How to create the tools directory](../../how-to/create-tools-dir.md)

Added in v1.5.

## --setfiles-context

The SELinux context to use when running the `setfiles` command, which is used to apply
the security context labels on all the files in the OS when SELinux is enabled in the
target OS.

If omitted, the `setfiles` command is run in the default security context chosen by the
kernel.

This option is useful when running on a build system with SELinux enabled.

Added in v1.5.

### selinux-policy (e.g. Fedora)

When running on a build system with SELinux enabled and that uses the
[selinux-policy](https://github.com/fedora-selinux/selinux-policy) policy (e.g. Fedora),
use the value: `system_u:system_r:setfiles_mac_t:s0`

The `setfiles_mac_t` label is allowed to use the `CAP_MAC_ADMIN` capability, which
permits SELinux labels to be applied that aren't present in the build host's SELinux
rules. This is super useful when customizing images that have a different distro /
distro version than the build host.

Also, you need to run the commands:

1. `sudo semanage permissive -a setfiles_mac_t`

   This allows transitions from `unconfined_t` to `setfiles_mac_t`.

2. `sudo semanage permissive -a systemd_hwdb_t`

   `setfiles_mac_t` doesn't have the correct permissions to label `/etc/udev/hwdb.bin`.

### refpolicy

When running on a build system that uses the
[refpolicy](https://github.com/SELinuxProject/refpolicy) policy (e.g. Azure Linux 3.0),
the only thing you can do is disable SELinux.

This SELinux policy does not have a label with permissions for the `CAP_MAC_ADMIN`
capability.
