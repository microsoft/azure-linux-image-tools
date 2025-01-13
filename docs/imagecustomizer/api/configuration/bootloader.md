# bootLoader type

Defines the configuration for the boot-loader.

## resetType [string]

Specifies that the boot-loader configuration should be reset and how it should be reset.

Supported options:

- `hard-reset`: Fully reset the boot-loader and its configuration.
  This includes removing any customized kernel command-line arguments that were added to
  base image.

Example:

```yaml
os:
  bootloader:
    resetType: hard-reset
```
