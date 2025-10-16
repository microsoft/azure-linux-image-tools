---
parent: Configuration
ancestor: Image Customizer
---

# ociPlatform type

Specifies the platform to use when the URI points to a multi-platform artifact.

Example:

```yaml
input:
  image:
    oci:
      uri: mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest
      platform:
        os: linux
        architecture: amd64
```

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v1.1.

## os [string]

The operating system of the artifact.

This is used to filter multi-platform OCI artifacts.

For example: `linux`

Added in v1.1.

## architecture [string]

The CPU architecture of the artifact.

This is used to filter multi-platform OCI artifacts.

For example:

- `amd64`
- `arm64`

Added in v1.1.
