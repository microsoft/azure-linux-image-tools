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

- `modify`: Modify only the UKI's own addon, `<uki-name>.extra.d/<kernel>.addon.efi`, to append or update kernel
  command-line arguments. The main UKI file (kernel, initramfs, os-release, systemd-stub, kernel command line) and the
  other addons remain unchanged. If the UKI has no such addon, one is created with just the new arguments.
  systemd-stub appends the addon's command line to the main UKI's, so an argument in the addon overrides the main
  UKI's value. A change that would remove an argument of the main UKI or another addon fails instead, such as
  `selinux.mode` from `force-enforcing` to `enforcing`, which drops `enforcing=1`; use `create` mode for it.

  **Restrictions for modify mode:**
  - Kernel and initramfs cannot be modified (package updates that change kernel/initramfs are not allowed)
  - Only kernel command-line arguments can be changed via:
    - `kernelCommandLine.extraCommandLine` (appended to existing args)
    - `selinux.mode` (replaces existing SELinux args)
    - `storage.reinitializeVerity` (replaces existing verity args)
  - On Azure Container Linux with A/B updates, `storage.reinitializeVerity` isn't supported, since the addon would
    override the root hash of the A/B slot addon.

  Prior to v1.8, modify mode required the UKIs to have addons.

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

Example (modify mode):

```yaml
# Modify kernel cmdline in existing UKI addon without touching the main UKI.
# This preserves the kernel, initramfs, and all other UKI sections.
os:
  uki:
    mode: modify
  
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

Example (modify mode with SELinux and verity update):

```yaml
# Update SELinux mode and refresh verity hashes while preserving main UKI.
storage:
  reinitializeVerity: all

os:
  uki:
    mode: modify
  
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
