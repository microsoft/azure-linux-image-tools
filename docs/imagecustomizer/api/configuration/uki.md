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

- `append`: Modify only the UKI addon to append or update kernel command-line arguments.
  The main UKI file (kernel, initramfs, os-release, systemd-stub) remains unchanged.
  This mode requires that the base image uses the UKI addon architecture where kernel
  command-line arguments are stored in a separate `.addon.efi` file rather than embedded
  in the main UKI.

  **Restrictions for append mode:**
  - Base image must have UKIs with addon architecture (`<uki-name>.extra.d/*.addon.efi`)
  - Kernel and initramfs cannot be modified (package updates that change kernel/initramfs are not allowed)
  - Only kernel command-line arguments can be changed via:
    - `kernelCommandLine.extraCommandLine` (appended to existing args)
    - `selinux.mode` (replaces existing SELinux args)
    - `storage.reinitializeVerity` (replaces existing verity args)

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

Example (append mode):

```yaml
# Modify kernel cmdline in existing UKI addon without touching the main UKI.
# This preserves the kernel, initramfs, and all other UKI sections.
os:
  uki:
    mode: append
  
  kernelCommandLine:
    extraCommandLine:
      - console=ttyS0
      - debug
  
  # You can still perform OS customizations:
  packages:
    install:
      - nginx
  
  additionalFiles:
    - path: /etc/app-config.txt
      content: |
        Application configuration

previewFeatures:
  - uki
```

Example (append mode with SELinux and verity update):

```yaml
# Update SELinux mode and refresh verity hashes while preserving main UKI.
storage:
  reinitializeVerity: all

os:
  uki:
    mode: append
  
  selinux:
    mode: permissive
  
  kernelCommandLine:
    extraCommandLine:
      - rd.info

previewFeatures:
  - uki
  - reinitialize-verity
```

Added in v1.2.0
