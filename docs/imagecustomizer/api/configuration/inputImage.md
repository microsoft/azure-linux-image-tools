---
parent: Configuration
ancestor: Image Customizer
---

# inputImage type

Specifies the configuration for the input image.

Only one child field (`path`, `oci`, or `azureLinux`) may be specified.

An input image must either be provided in the configuration file or on the command line
(e.g. [--image-file](../cli/cli.md#--image-filefile-path)).
If both a command-line input image and a configuration input image are specified, then
the command line's input image overrides the config file's input image.

Example:

```yaml
input:
  image:
    path: ./base.vhdx
```

## path [string]

The base image file to customize.

This file is typically one of the standard Azure Linux core images.
But it can also be an Azure Linux image that has been customized.

Supported image file formats: vhd, vhdx, qcow2, and raw.

Note: Image Customizer will reject VHD files created by `qemu-img` unless the
`-o force_size=on` option was passed. Without this option, `qemu-img` will
likely change the size of the disk (to a non-multiple of 1 MiB), which can cause
problems when trying to upload the disk to Azure.

If verity is enabled in the base image, then:

- If the partitions are recustomized using the
  [disks](storage.md#disks-disk) API, then the existing verity
  settings are thrown away.
  New verity settings can be configured with the
  [verity](verity.md) API.

- Otherwise, the existing verity settings are reapplied to the image after OS
  customization, according to the
  [.storage.reinitializeVerity](storage.md#reinitializeverity-string)
  setting.

  This feature is in preview and may be subject to breaking changes.
  You may enable this feature by adding `reinitialize-verity` to the
  [previewFeatures](config.md#previewfeatures-string) API or
  via the [--preview-feature](../cli/cli.md#--preview-featurefeature) flag.

Added in v0.13.

## oci [[ociImage](ociimage.md)]

Download the base image file from an OCI artifact.

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewFeatures](../configuration/config.md#previewfeatures-string) API or
via the [--preview-feature](../cli/cli.md#--preview-featurefeature) flag.

Added in v1.1.

## azureLinux [[azureLinuxImage](azurelinuximage.md)]

Download an Azure Linux image file to use as the base image.

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewFeatures](../configuration/config.md#previewfeatures-string) API or
via the [--preview-feature](../cli/cli.md#--preview-featurefeature) flag.

Added in v1.1.
