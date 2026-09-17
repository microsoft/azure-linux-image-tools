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
  valid `os.bootloader.resetType` value of `hard-reset` is required when `os.uki` is
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

- `kdump-boot-files`: Enables support for crash dump configuration.

  When this option is specified, the
  [`iso.kdumpBootFiles`](./iso.md#kdumpbootfiles-kdumpbootfiles) and
  [`pxe.kdumpBootFiles`](./pxe.md#kdumpbootfiles-kdumpbootfiles) configurations
  become available.

  Added in v0.16.

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

- `btrfs`: Enables support for creating BTRFS file systems.

  When this option is specified, the `btrfs` option for [storage.filesystems[].type](./filesystem.md#type-string)
  and the [storage.filesystems[].btrfs](./filesystem.md#btrfs-btrfsconfig) become available.

  Added in v1.2.

- `create`: Enables the [create subcommand](../cli/create.md) for building new images from scratch.

  Added in v1.2.

- `preview-distro-version`: Enables support for distros and distro versions that are still in
  preview, including Fedora and Ubuntu (via the [customize](../cli/customize.md) and
  [create](../cli/create.md) subcommands), Azure Container Linux, and Azure Linux 4.0.

  Added in v1.5.

- `unsupported-distro-version`: Enables support for customizing an unsupported distro
  version.

  Added in v1.5.

- `tools-dir`: Enables support for the
  [--tools-dir](../cli/customize.md#--tools-dirdirectory-path) API.

  Added in v1.5.

- `acl-grow-partitions`: Enables the narrow, Azure Container Linux (ACL) only API for growing
  ACL's standard partitions (e.g. `/usr`, ESP) to explicit target sizes.

  When this option is specified, the `acl.usr` and `acl.esp` configuration becomes available. It is
  only valid for ACL target images.

  See [acl](./acl.md) for more details.

  Added in v1.6.

- `acl-oem-id`: Enables the narrow, Azure Container Linux (ACL) only API for overriding the flatcar
  OEM id (`flatcar.oem.id`) on the boot kernel command line.

  When this option is specified, the `acl.oemId` configuration becomes available. It is only valid
  for ACL target images.

  See [acl](./acl.md) for more details.

- `remove-package-manager`: Enables support for the
  ([os.packages.removePackageManager](./packages.md#removepackagemanager-bool)) API.

  Added in v1.6.

## output [[output](./output.md)]

Specifies the configuration for the output image and artifacts.

Added in v0.13.


## baseConfigs [[baseConfig](./baseConfig.md)]

Specifies a list of configuration files to inherit from.

Added in v1.1.

## acl [[acl](./acl.md)]

Narrow, Azure Container Linux (ACL) only configuration: grows ACL's standard partitions to explicit
target sizes and/or overrides the boot OEM id. Gated behind the `acl-grow-partitions` and
`acl-oem-id` preview features.

Added in v1.6.
