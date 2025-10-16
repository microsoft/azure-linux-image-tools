---
parent: Configuration
ancestor: Image Customizer
---

# ociImage type

A reference to an OCI artifact containing an OS image file.

The OCI artifact is expected to have a file with one of the following file extensions:

- `.vhdx`
- `.vhd`
- `.qcow2`
- `.img`
- `.raw`

If multiple such files exists in the artifact, then an error will occur.

Example:

```yaml
input:
  image:
    oci:
      uri: mcr.microsoft.com/azurelinux/3.0/image/minimal-os:latest
      platform: linux/amd64
```

This feature is in preview and may be subject to breaking changes.
You may enable this feature by adding `input-image-oci` to the
[previewfeatures](../configuration/config.md#previewfeatures-string) API.

Added in v1.1.

## uri [string]

The URI of the OCI artifact containing the image to use as the base image.

Added in v1.1.

## platform [[ociPlatform](ociplatform.md)]

Specifies the platform to use when the URI points to a multi-platform artifact.

As syntactic sugar, this value can be specified as a string with the following format:

  `OS[/ARCH]`

Where:

- `OS`: The OS value.
- `ARCH`: The CPU architecture. If not specified, then it defaults to the system's
  CPU architecture.

Added in v1.1.
