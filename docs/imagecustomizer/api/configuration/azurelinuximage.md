---
parent: Configuration
ancestor: Image Customizer
---

# azureLinuxImage type

Specifies an Azure Linux image to be used as the base image.

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Example:

```yaml
previewFeatures:
- input-image-oci

input:
  image:
    azureLinux:
      variant: minimal-os
      version: 3.0
```

The Azure Linux images are downloaded from the
[Microsoft Artifact Registry](https://mcr.microsoft.com) (MCR).

The URI used is:

| Version      | URI                                                             |
| ------------ | --------------------------------------------------------------- |
| 2.0          | mcr.microsoft.com/azurelinux/2.0/image/`<VARIANT>`:latest       |
| 3.0          | mcr.microsoft.com/azurelinux/3.0/image/`<VARIANT>`:latest       |
| 2.0.`<DATE>` | mcr.microsoft.com/azurelinux/2.0/image/`<VARIANT>`:2.0.`<DATE>` |
| 3.0.`<DATE>` | mcr.microsoft.com/azurelinux/3.0/image/`<VARIANT>`:3.0.`<DATE>` |

The list of Azure Linux image versions is available on MCR.
For example: the
[Azure Linux 3.0 minimal-os versions](https://mcr.microsoft.com/en-us/artifact/mar/azurelinux/3.0/image/minimal-os/tags).

Added in v1.1.

## version [string]

Specifies the version of the Azure Linux image to use as the base image.

This value is required.

Supported values include:

- `2.0`
- `3.0`
- `2.0.<DATE>` (e.g. `2.0.20251010`)
- `3.0.<DATE>` (e.g. `3.0.20250910`)

Added in v1.1.

## variant [[ociPlatform](ociplatform.md)]

Specifies the variant of the Azure Linux image to use as the base image.

This value is required.

Example values:

- `minimal-os`

Added in v1.1.
