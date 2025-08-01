---
parent: Configuration
ancestor: Image Customizer
---

# injectFilesConfig type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `inject-files` in the
[previewFeatures](#previewfeatures-string) API.

Specifies the configuration for injecting files into specified partitions of
an image.

This file is typically generated automatically by Prism when the
[`output.artifacts`](./outputArtifacts.md) feature is used. The generated file
is named `inject-files.yaml` and placed under the specified output directory.
You can modify this file if needed (e.g., to add IPE policies or customize
destinations), or you may also create this file from scratch. And later use it
with the [`inject-files` CLI command](../cli.md#inject-files) to perform the
injection.

Example:

```yaml
injectFiles:
- partition:
    idType: part-uuid
    id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
  destination: /EFI/Linux/vmlinuz-6.6.51.1-5.azl3.efi
  source: ./vmlinuz-6.6.51.1-5.azl3.signed.efi
  unsignedSource: ./vmlinuz-6.6.51.1-5.azl3.unsigned.efi
- partition:
    idType: part-uuid
    id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
  destination: /EFI/BOOT/bootx64.efi
  source: ./bootx64.signed.efi
  unsignedSource: ./bootx64.efi
- partition:
    idType: part-uuid
    id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
  destination: /EFI/systemd/systemd-bootx64.efi
  source: ./systemd-bootx64.signed.efi
  unsignedSource: ./systemd-bootx64.efi
- partition:
    idType: part-uuid
    id: 5c0a7f80-0f9f-48f6-8bb1-d622022aaf24
  destination: /root.hash.sig
  source: ./root.hash.sig
  unsignedSource: ./root.hash
previewFeatures:
- inject-files
```

Added in v0.14.

## injectFiles [`InjectArtifactMetadata`](./injectArtifactMetadata.md)[]

Required.

Specifies a list of files to inject into specific partitions of the image.

Each item in this list must follow the structure defined in the
[`InjectArtifactMetadata`](./injectArtifactMetadata.md) type.

Added in v0.14.

## previewFeatures [string[]]

Enables preview features.

Preview features are features that have not yet been stabilized.
Their APIs and behavior are subject to change.

Supported options:

- `inject-files`: Enables support for injecting files into specific partitions
  using a configuration file.

  When this option is specified, the `inject-files.yaml` configuration becomes
  available. This file can be generated using the `output.artifacts` API and
  later consumed via the `inject-files` CLI command.

  Added in v0.14.
