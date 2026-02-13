---
parent: Configuration
ancestor: Image Customizer
---

# config type

The top-level type of the configuration.

Added in v0.3.

## input [[input](./input.md)]

Specifies the configuration for the input image.

Added in v0.13.

## storage [[storage](./storage.md)]

Contains the options for provisioning disks, partitions, and file systems.

Customizing Ubuntu images using this API is not currently tested or supported.

Added in v0.3.

## iso [[iso](./iso.md)]

Optionally specifies the configuration for the generated ISO media.

Customizing Ubuntu images using this API is not currently tested or supported.

Added in v0.3.

## pxe [[pxe](./pxe.md)]

Optionally specifies the PXE-specific configuration for the generated OS artifacts.

Customizing Ubuntu images using this API is not currently tested or supported.

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
  [`--package-snapshot-time`](../cli/customize.md#--package-snapshot-time) CLI option or
  [`os.packages.snapshotTime`](./packages.md#snapshottime-string) API field.
  If both are provided, the CLI value takes precedence.

  Added in v0.15.

- `base-configs`: Enables support for hierarchical configuration inheritance.

  When this option is specified, the `baseConfigs` configuration becomes available.
  This allows configurations to inherit from one or more base configurations.

  See [Base Config](./baseConfig.md) for more details.

  Added in v1.1.

- `input-image-oci`: Enables downloading the base image from a OCI artifact.

  Added in v1.1.

- `output-selinux-policy`: Enables extraction of SELinux policy files from the
  customized image.

  When this option is specified, the `output.selinuxPolicyPath` configuration
  becomes available.

  See [output.selinuxPolicyPath](./output.md#selinuxpolicypath-string) for more details.

  Added in v1.1.

- `cosi-compression`: Enables custom compression settings for COSI output images.

  When this option is specified, the `output.image.cosi.compression.level` configuration
  and the `--cosi-compression-level` CLI flag become available.

  See [cosiCompression](./cosiCompression.md) for more details.

  Added in v1.2.

- `btrfs`: Enables support for creating BTRFS file systems.

  When this option is specified, the `btrfs` option for [storage.filesystems[].type](./filesystem.md#type-string)
  and the [storage.filesystems[].btrfs](./filesystem.md#btrfs-btrfsconfig) become available.

  Added in v1.2.

## output [[output](./output.md)]

Specifies the configuration for the output image and artifacts.

Added in v0.13.


## baseConfigs [[baseConfig](./baseConfig.md)]

Specifies a list of configuration files to inherit from.

Added in v1.1.
