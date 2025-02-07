---
parent: Configuration
---

# config type

The top-level type of the configuration.

## storage [[storage](./storage.md)]

Contains the options for provisioning disks, partitions, and file systems.

## iso [[iso](./iso.md)]

Optionally specifies the configuration for the generated ISO media.

## pxe [[pxe](./pxe.md)]

Optionally specifies the PXE-specific configuration for the generated OS artifacts.

## os [[os](./os.md)]

Contains the configuration options for the OS.

Example:

```yaml
os:
  hostname: example-image
```

## scripts [[scripts](./scripts.md)]

Specifies custom scripts to run during the customization process.

## previewFeatures [string[]]

Enables experimental and preview features that are not yet generally available.
Features listed under previewFeatures must be explicitly included in the Image
Customizer configuration to enable their usage.

Supported options:

- `uki`: Enables the Unified Kernel Image (UKI) feature.

  When this option is specified, The `os.uki` configuration becomes available. A
  valid `os.bootloader.reset` value of `hard-reset` is required when `os.uki` is
  configured.

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
