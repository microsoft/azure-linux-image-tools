---
parent: Configuration
ancestor: Image Creator
---

# config type

The top-level type of the configuration.

## storage [[storage](../../../imagecustomizer/api/configuration/storage.md)]

Contains the options for provisioning disks, partitions, and file systems.

You should specify the disks, partitions and filesystems for creating a image these cannot be empty.

For supported fields in the storage level of the configuration refer to
[schema](../../api/configuration/configuration.md#schema-overview)

## os [[os](../../../imagecustomizer/api/configuration/os.md)]

Contains the configuration options for the OS.

For supported fields in the os level of the configuration refer to
[schema](../../api/configuration/configuration.md#schema-overview)

Example:

```yaml
os:
  hostname: example-image
```

## scripts [[scripts](../../../imagecustomizer/api/configuration/scripts.md)]

Specifies custom scripts to run during the image creation process.

## previewFeatures [string[]]

Enables preview features.

Preview features are features that have not yet been stabilized.
Their APIs and behavior are subject to change.

Supported options:

- `package-snapshot-time`: Enables snapshot-based package filtering during image customization. See
  the detailed definition in the section below.

  Example:

  ```yaml
  os:
  previewFeatures:
  - package-snapshot-time
  ```

- `package-snapshot-time`: Enables snapshot-based package filtering during image
  customization. This allows specifying a cutoff timestamp using the
  [`--package-snapshot-time`](../../../imagecustomizer/api/cli/cli.md#--package-snapshot-time) CLI option or
  [`os.packages.snapshotTime`](../../../imagecustomizer/api/configuration/packages.md#snapshottime-string) API field.
  If both are provided, the CLI value takes precedence.

## output [[output](../../../imagecustomizer/api/configuration/output.md#image-outputimage)]

Specifies the configuration for the output image.
