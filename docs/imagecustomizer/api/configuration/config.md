---
parent: Configuration
---

# config type

The top-level type of the configuration.

Added in v0.3.

## input [[input](./input.md)]

Specifies the configuration for the input image.

Added in v0.13.0.

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

Enables experimental and preview features that are not yet generally available.
Features listed under previewFeatures must be explicitly included in the Image
Customizer configuration to enable their usage.

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

- `inject-files`: Enables support for injecting files into specific partitions
  using a configuration file.

  When this option is specified, the `inject-files.yaml` configuration becomes
  available. This file can be generated using the `output.artifacts` API and
  later consumed via the `inject-files` CLI command.

  See [`injectFilesConfig`](./injectFilesConfig.md) for more details.

  Added in v0.14.

- `reinitialize-verity`: Enables support for customizing an image that has verity
  enabled (without needing to recustomize the partitions). The verity settings are read
  from the image and reapplied after OS customization.

  Added in v0.15.

## output [[output](./output.md)]

Specifies the configuration for the output image and artifacts.

Added in v0.13.
