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
  bootloader:
    resetType: hard-reset
  uki:
    mode: create
previewFeatures:
- uki
```

Added in v0.8.

## mode [string]

Specifies how to handle UKI creation or preservation.

Required.

Supported values:

- `create`: Create UKI files for all installed kernels. When used with a base image that
  already has UKIs, the new UKIs will be generated and override the old ones.

- `passthrough`: Preserve existing UKI files without modification.

Example (creating UKIs):

```yaml
os:
  bootLoader:
    resetType: hard-reset
  uki:
    mode: create
  
  kernelCommandLine:
    extraCommandLine:
      - rd.info
  
previewFeatures:
  - uki
```

Example (passthrough mode):

```yaml
# Customize an existing UKI image without regenerating UKI files.
# This preserves the existing kernel, initramfs, and cmdline in the UKI.
os:
  uki:
    mode: passthrough
  
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

Example (re-customizing UKI with verity):

```yaml
# Recustomize an existing UKI+verity image with updated verity hashes.
storage:
  reinitializeVerity: all

os:
  bootloader:
    resetType: hard-reset
  
  uki:
    mode: create
  
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

Added in v1.2.0
