# previewFeatures

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
