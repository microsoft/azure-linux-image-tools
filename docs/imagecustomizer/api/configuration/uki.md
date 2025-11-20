---
parent: Configuration
ancestor: Image Customizer
---

# uki type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `uki` in the
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

## reinitialize [string]

Controls the behavior when customizing an image that already contains UKI files.

Value is optional.

This field is only relevant when customizing an existing image that already has
UKI files in the `/boot/efi/EFI/Linux` directory. It determines whether to
preserve the existing UKI files or regenerate them during the customization process.

Supported options:

- (unspecified): When `reinitialize` is not specified, the tool generates new UKI
  files based on the kernel and initramfs files present in the image. The `kernels`
  field must be specified to indicate which kernels to build UKIs for. This is the
  default behavior for first-time UKI creation.

- `passthrough`: Preserve existing UKI files without modification. The kernel,
  initramfs, and command-line arguments embedded in the existing UKIs are left
  unchanged. When this option is specified, the `kernels` field must not be
  specified (since no new UKIs are being generated).

- `refresh`: Extract the kernel and initramfs from existing UKI files, then
  regenerate new UKI files with updated configurations. The `kernels` field
  must be specified to indicate which kernels to build UKIs for.

Example (passthrough mode):

```yaml
# Customize an existing UKI image without regenerating UKI files.
# This preserves the existing kernel, initramfs, and cmdline in the UKI.
os:
  uki:
    reinitialize: passthrough
  
  # You can still perform OS customizations:
  packages:
    install:
      - nginx
      - vim
  
  additionalFiles:
    - path: /etc/app-config.txt
      content: |
        Application configuration

previewFeatures:
  - uki
```

Example (refresh mode with verity):

```yaml
# Recustomize an existing UKI+verity image with updated verity hashes.
# Extracts kernel/initramfs from existing UKIs and rebuilds them with
# updated kernel command-line arguments (including new verity hashes).
storage:
  reinitializeVerity: all

os:
  bootloader:
    resetType: hard-reset
  
  uki:
    kernels: auto
    reinitialize: refresh
  
  kernelCommandLine:
    extraCommandLine:
      - rd.info
  
  packages:
    install:
      - openssh-server
  
  additionalFiles:
    - path: /etc/uki-recustomization.txt
      content: |
        UKI recustomization with verity refresh

previewFeatures:
  - uki
  - reinitialize-verity
```

Added in v1.0.1
