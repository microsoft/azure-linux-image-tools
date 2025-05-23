---
parent: Configuration
---

# uki type

This is a preview feature.
Its API and behavior is subject to change.
You must enabled this feature by specifying `uki` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Enables the creation of Unified Kernel Images (UKIs) and configures systemd-boot
to add UKIs as boot entries. UKI combines the Linux kernel, initramfs, kernel
command-line arguments, etc. into a single EFI executable, simplifying system
boot processes and improving security.

If this type is specified, then [os.bootloader.resetType](./bootloader.md#resettype-string)
must also be specified.

Example:

```yaml
os:
  bootLoader:
    resetType: hard-reset
  uki:
    kernels: auto
previewFeatures:
- uki
```

Added in v0.8.

## kernels

Specifies which kernels to produce UKIs for.

The value can either contain:

- The string `"auto"`
- A list of kernel version strings.

When `"auto"` is specified, the tool automatically searches for all the
installed kernels and produces UKIs for all the found kernels.

If a list of kernel versions is provided, then the tool will only produce UKIs
for the kernels specified.

The kernel versions must match the regex: `^\d+\.\d+\.\d+(\.\d+)?(-[\w\-\.]+)?$`.
Examples of valid kernel formats: `6.6.51.1-5.azl3`, `5.10.120-4.custom`, `4.18.0-80.el8`.

Example:

```yaml
os:
  uki:
    kernels: auto
```

Example:

```yaml
os:
  uki:
    kernels:
      - 6.6.51.1-5.azl3
      - 5.10.120-4.custom
```

Added in v0.8.
