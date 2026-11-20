---
parent: Configuration
ancestor: Image Customizer
---

# resizePartitionRef type

Specifies how to locate a partition in the base image.

Example:

```yaml
previewFeatures:
- resize-disk

storage:
  resizeDisk:
    partitions:
    - ref:
        idType: part-label
        id: ROOT
      size: 1G
```

Added in v1.8.

## idType [string]

Required.

Specifies the type of identifier used to find the partition.

Supported values:

- `last`: The last partition on the disk (according to the disk's physical
   layout, not the partition table ordering).
- `part-label`: Search by partition label.
- `label`: : Search by filesystem label.
- `part-uuid`: Search by partition UUID.

Added in v1.8.

## id [string]

Required.

The identifier value of the partition, interpreted according to
[idType](#idtype-string).

Added in v1.8.
