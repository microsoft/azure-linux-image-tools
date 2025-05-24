---
parent: Configuration
---

# InjectFilePartition type

This is a preview feature.
Its API and behavior is subject to change.
You must enabled this feature by specifying `inject-files` in the
[previewFeatures](./injectFilesConfig.md#previewfeatures-string) API.

Defines how to locate the target partition where an artifact should be injected.

This object is used as the `partition` field in each entry of the
[`InjectArtifactMetadata`](./injectArtifactMetadata.md) list.

Added in v0.14.

## Example

```yaml
idType: part-uuid
id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
```

## `idType` [MountIdentifierType]

Required.

Specifies the type of identifier used to find the partition.

Accepted values:

- `uuid` – Filesystem UUID of the partition
- `part-uuid` – Partition UUID (GPT/MBR PARTUUID)
- `part-label` – Partition label (GPT PARTLABEL)
- *(empty string)* – Defaults to `part-uuid`

For most use cases, `part-uuid` is recommended.

Added in v0.14.

## `id` [string]

Required.

The identifier value of the partition, interpreted according to `idType`.

Added in v0.14.
