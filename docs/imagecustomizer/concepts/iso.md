---
parent: Concepts
title: ISO Support
nav_order: 3
---

# Image Customizer Live OS ISO Support

## Overview

The Image Customizer tool can customize an input image and package the output
as a [Live OS](./liveos.md) iso image. A Live OS iso image is a bootable image
that boots into a root file system included on the iso media without the need
to have anything pre-installed on the target machine.

## Creating a Live OS ISO

The input image can be a full disk image (vhd/vhdx/qcow2/raw) or previously
generated Live OS iso image.

To generate a Live OS iso, set the `--output-image-format` parameter to `iso`.
More info can be found at
[Creating a LiveOS ISO how-to guide](../how-to/live-iso.md)

For a full list of capabilities, see [ISO configuration](../api/configuration/iso.md)
page

## cloud-init Support

In some user scenarios, it desired to embed the cloud-init data files into the
iso media. The easiest way is to include the data files on the media, and then
the cloud-init `ds` kernel parameter to where the files are.

The files can be placed directly within the iso file system or they can be
placed within the LiveOS root file system.

Placing those files directly on the iso file system will allow a more efficient
replacement flow in the future (i.e. when it is desired to only replace the
cloud-init data files).

### Examples

#### Example 1

Placing cloud-init data directly within the iso file system:

```yaml
scripts:
  postCustomization:
  - content: |
      set -e
      mkdir -p /var/lib/cloud/seed/
      ln -s -T /run/initramfs/live/cloud-init-data /var/lib/cloud/seed/nocloud

iso:
  additionalFiles:
    cloud-init-data/user-data: /cloud-init-data/user-data
    cloud-init-data/network-config: /cloud-init-data/network-config
    cloud-init-data/meta-data: /cloud-init-data/meta-data

  kernelCommandLine:
    extraCommandLine:
    - "ds=nocloud"
```

Note: It is tempting to specify
`extraCommandLine: "'ds=nocloud;seedfrom=file://run/initramfs/live/cloud-init-data'"`,
instead of using a symbolic link.
But cloud-init ignores the `network-config` file when you use `seedfrom`.
See, cloud-init issue [#3307](https://github.com/canonical/cloud-init/issues/3307).

#### Example 2

Placing the cloud-init data within the LiveOS root file system:

```yaml
os:
  kernelCommandLine:
    extraCommandLine:
    - "ds=nocloud"

  additionalFiles:
    cloud-init-data/user-data: /var/lib/cloud/seed/nocloud/user-data
    cloud-init-data/network-config: /var/lib/cloud/seed/nocloud/network-config
    cloud-init-data/meta-data: /var/lib/cloud/seed/nocloud/meta-data
```
