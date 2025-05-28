---
parent: Configuration
---

# config type

The top-level type of the configuration.

Added in v0.3.

## input [[input](./input.md)]

Specifies the configuration for the input image.

Added in v0.13.

## storage [[storage](./storage.md)]

Contains the options for provisioning disks, partitions, and file systems.

Added in v0.3.

## iso [[iso](./iso.md)]

Optionally specifies the configuration for the generated ISO media.

Added in v0.3.

## pxe [[pxe](./pxe.md)]

Optionally specifies the PXE-specific configuration for the generated OS artifacts.

Added in v0.8.

## os [[os](./os.md)]

Contains the configuration options for the OS.

Example:

```yaml
os:
  hostname: example-image
```

Added in v0.3.

## scripts [[scripts](./scripts.md)]

Specifies custom scripts to run during the customization process.

Added in v0.3.

## previewFeatures [string[]]

Enables preview features.

Preview features are features that have not yet been stabilized.
Their APIs and behavior are subject to change.

Supported options:

- `uki`: Enables the Unified Kernel Image (UKI) feature.

  When this option is specified, the `os.uki` configuration becomes available. A
  valid `os.bootloader.reset` value of `hard-reset` is required when `os.uki` is
  configured.

  Added in v0.8.

  Example:

  ```yaml
  os:
    bootloader:
      resetType: hard-reset
    uki:
      kernels: auto
  previewFeatures:
  - uki
  ```

- `output-artifacts`: Enables the configuration for the output directory
  containing the generated artifacts.

  When this option is specified, the `output.artifacts` configuration becomes available.

  See [Output Artifacts](./outputArtifacts.md) for more details.

  Added in v0.14.

- `reinitialize-verity`: Enables support for customizing an image that has verity
  enabled (without needing to recustomize the partitions). The verity settings are read
  from the image and reapplied after OS customization.

  Added in v0.15.

- `package-snapshot-time`: Enables snapshot-based package filtering during image
  customization. This allows specifying a cutoff timestamp using the
  [`--package-snapshot-time`](../cli.md#--package-snapshot-time) CLI option or
  [`os.packages.snapshotTime`](./packages.md#snapshottime-string) API field.
  If both are provided, the CLI value takes precedence.

  Added in v0.15.

## output [[output](./output.md)]

Specifies the configuration for the output image and artifacts.

Added in v0.13.
