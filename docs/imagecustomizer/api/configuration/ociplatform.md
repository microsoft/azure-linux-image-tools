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
[previewFeatures](../configuration/config.md#previewfeatures-string) API or
via the [--preview-feature](../cli/cli.md#--preview-featurefeature) flag.

Added in v1.1.

## os [string]

The operating system of the artifact.

This is used to filter multi-platform OCI artifacts.

This value can technically be anything.
However, by convention, values typically use the same names as Go's
[runtime.GOOS](https://pkg.go.dev/runtime#pkg-constants) constant.

For example: `linux`

Added in v1.1.

## architecture [string]

The CPU architecture of the artifact.

This is used to filter multi-platform OCI artifacts.

This value can technically be anything.
However, by convention, values typically use the same names as Go's
[runtime.GOARCH](https://pkg.go.dev/runtime#pkg-constants) constant.

For example:

- `amd64`
- `arm64`

Added in v1.1.
