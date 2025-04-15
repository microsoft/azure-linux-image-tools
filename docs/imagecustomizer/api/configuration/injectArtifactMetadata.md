---
parent: Configuration
---

# InjectArtifactMetadata type

Defines a single artifact to be injected into a partition during image modification.

This is used in the [`InjectFilesConfig`](./injectFilesConfig.md) array
when performing injection via the [`inject-files` CLI command](../cli/inject-files.md).

## Example

```yaml
partition:
  idType: part-uuid
  id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
destination: /EFI/BOOT/bootx64.efi
source: ./bootx64.signed.efi
unsignedSource: ./bootx64.efi
```

## `partition` [InjectFilePartition](./injectFilePartition.md)

Required.

The target partition where the artifact should be injected.

This field must be an object of type [`InjectFilePartition`](./injectFilePartition.md), with fields:

- `idType`: How the partition should be identified. Options:
  - `uuid`
  - `part-uuid`
  - `part-label`
- `id`: The identifier value (such as the GPT partition UUID or label).

## `destination` [string]

Required.

The absolute path (inside the target partition) where the artifact should be copied.

For example: `/EFI/BOOT/bootx64.efi`

## `source` [string]

Required.

Path to the signed artifact file to be injected. This path may be relative to the `inject-files.yaml` config file or an absolute path.

## `unsignedSource` [string]

Optional.

Path to the original unsigned artifact (if available). This field is for informational or auditing purposes only — it is not used during injection.

Added in v0.14.0
