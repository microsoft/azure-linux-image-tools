# Configuration

Image customizations can be configured using a YAML (or JSON) file.

## Schema Overview

- [Configuration](#configuration)
  - [Schema Overview](#schema-overview)
    - [iso \[iso\]](#iso-iso)
    - [os \[os\]](#os-os)
    - [pxe \[pxe\]](#pxe-pxe)
    - [scripts \[scripts\]](#scripts-scripts)
    - [storage \[storage\]](#storage-storage)
  - [Operation ordering](#operation-ordering)
  - [/etc/resolv.conf](#etcresolvconf)
  - [Replacing packages](#replacing-packages)

### iso [[iso](./iso.md)]

Optionally specifies the configuration for the generated ISO media.

### os [[os](./os.md)]

Contains the configuration options for the OS.

Example:

```yaml
os:
  hostname: example-image
```
### pxe [[pxe](./pxe.md)]
Optionally specifies the PXE-specific configuration for the generated OS
artifacts.
### scripts [[scripts](./scripts.md)]
Specifies custom scripts to run during the customization process.
### storage [[storage](./storage.md)]
## Operation ordering
1. If partitions were specified in the config, customize the disk partitions.
   Otherwise, if the [resetpartitionsuuidstype](#resetpartitionsuuidstype-string) value
   is specified, then the partitions' UUIDs are changed.
2. Override the `/etc/resolv.conf` file with the version from the host OS.

3. Update packages:

   1. Remove packages ([removeLists](#removelists-string),
   [remove](#remove-string))

   2. Update base image packages ([updateExistingPackages](#updateexistingpackages-bool)).

   3. Install packages ([installLists](#installlists-string),
   [install](#install-string))

   4. Update packages ([updateLists](#removelists-string),
   [update](#update-string))

4. Update hostname. ([hostname](#hostname-string))

5. Copy additional files. ([additionalFiles](#os-additionalfiles))

6. Copy additional directories. ([additionalDirs](#additionaldirs-dirconfig))

7. Add/update users. ([users](#users-user))

8. Enable/disable services. ([services](#services-type))

9. Configure kernel modules. ([modules](#modules-module))

10. Write the `/etc/image-customizer-release` file.

11. If the bootloader [resetType](#resettype-string) is set to `hard-reset`, then
    reset the boot-loader.

    If the bootloader [resetType](#resettype-string) is not set, then append the
    [extraCommandLine](#extracommandline-string) value to the existing

12. Update the SELinux mode. [mode](#mode-string)

13. If ([overlays](#overlay-type)) are specified, then add the overlay driver
    and update the fstab file with the overlay mount information.

14. If a ([verity](#verity-type)) device is specified, then add the dm-verity dracut
    driver and update the grub config.

15. Regenerate the initramfs file (if needed).

16. Run ([postCustomization](#postcustomization-script)) scripts.

17. Restore the `/etc/resolv.conf` file.

18. If SELinux is enabled, call `setfiles`.

19. Run finalize image scripts. ([finalizeCustomization](#finalizecustomization-script))

20. If [--shrink-filesystems](./cli.md#shrink-filesystems) is specified, then shrink
    the file systems.

21. If a ([verity](#verity-type)) device is specified, then create the hash tree and
    update the grub config.

22. If the output format is set to `iso`, copy additional iso media files.
    ([iso](#iso-type))

23. If [--output-pxe-artifacts-dir](./cli.md#output-pxe-artifacts-dir) is specified,
    then export the ISO image contents to the specified folder.

## /etc/resolv.conf

The `/etc/resolv.conf` file is overridden during customization so that the package
installation and customization scripts can have access to the network.

Near the end of customization, the `/etc/resolv.conf` file is restored to its original
state.

However, if the `/etc/resolv.conf` did not exist in the base image and
`systemd-resolved` service is enabled, then the `/etc/resolv.conf` file is symlinked to
the `/run/systemd/resolve/stub-resolv.conf` file. (This would happen anyway during
first-boot. But doing this during customization is useful for verity enabled images
where the filesystem is readonly.)

If you want to explicitly set the `/etc/resolv.conf` file contents, you can do so within
a [finalizeCustomization](#finalizecustomization-script) script, since those scripts run
after the `/etc/resolv.conf` is deleted.

## Replacing packages

If you wish to replace a package with conflicting package, then you can remove the
existing package using [remove](#remove-string) and then install the
new package with [install](#install-string).

Example:

```yaml
os:
  packages:
    remove:
    - kernel
    install:
    - kernel-uvm
```
