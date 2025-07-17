---
parent: Configuration
ancestor: Image Customizer
---

# verityPartition type

Specifies how to locate a partition in the base image.

Example:

```yaml
storage:
  verity:
  - id: verityusr
    name: usr
    dataDevice:
      idType: part-label
      id: usr
    hashDevice:
      idType: part-label
      id: usrhash
    corruptionOption: panic
```

Added in v0.13.

## idType [string]

Required.

Specifies the type of identifier used to find the partition.

Supported values:

- `part-label`: Search by partition label.

Added in v0.13.

## id [string]

Required.

The identifier value of the partition, interpreted according to [idType](#idtype-string).

Added in v0.13.
