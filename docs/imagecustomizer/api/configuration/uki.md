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

Required.

Specifies how to handle UKI creation or preservation.

Supported values:

- `create`: Create UKI files for all installed kernels. When used with a base image that
  already has UKIs, the new UKIs will be generated and override the old ones.

  A kernel installed during the same run, for example when the kernel package is swapped, has no kernel command line of
  its own to carry forward. It inherits the command line that the image's other kernels share. Customization fails if
  those kernels have divergent command lines, since there is then no single correct value to inherit.

  The kernel command line goes into the main UKI. If the base image already has UKIs, the new UKIs keep their layout
  (unless `os.bootloader.resetType` is `hard-reset`): each kernel argument stays in the main UKI or the addon in
  `<uki-name>.extra.d/` it came from, also when its value changes. A new argument goes into the main UKI, or after the
  last addon argument of the same name, which it overrides. Other files in `<uki-name>.extra.d/` are kept. Since the
  UKIs are rebuilt from that layout, customization fails if they change during the run; add kernel arguments with
  `os.kernelCommandLine.extraCommandLine`.

  On Azure Container Linux with A/B updates, the A/B slot addon's arguments can't change apart from the root hash,
  which is also updated in the slot addon templates under `acl/uki-addons/` (rebuilt unsigned, like the UKIs).

  Prior to v1.8, the kernel command line went into `<uki-name>.extra.d/<kernel>.addon.efi` (on Azure Container Linux,
  apart from `flatcar.first_boot=detected`), and the other files in `<uki-name>.extra.d/` were removed. The arguments
  of a UKI built that way stay in that addon.

- `passthrough`: Preserve existing UKI files without modification.

Prior to v1.8, a `modify` value rebuilt only `<uki-name>.extra.d/<kernel>.addon.efi`. Use `create` instead.

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
