---
parent: Configuration
---

# module type

Options for configuring a kernel module.

## name [string]

The name of the kernel module to configure.

## loadMode [string]

The loadMode setting for kernel modules dictates how and when these modules
are loaded or disabled in the system.

Supported values:

- `always`: Set kernel modules to be loaded automatically at boot time.

  - If the module is disabled in the base image, then remove the blacklist entry from
    the `/etc/modprobe.d/modules-disabled.conf` file.

  - Add the module to `/etc/modules-load.d/modules-load.conf`.

- `auto`: Used for modules that are automatically loaded by the kernel as needed,
    without explicit configuration to load them at boot.

  - If the module is disabled in the base image, then remove the blacklist entry from
    the `/etc/modprobe.d/modules-disabled.conf` file.

- `disable`: Configures kernel modules to be explicitly disabled, preventing them from
  loading automatically.

  - If the module is not already disabled in the base image, then add a blacklist entry
    to the `/etc/modprobe.d/modules-disabled.conf` file.

- `inherit`: Configures kernel modules to inherit the loading behavior set in the base
  image.

Default value: `inherit`.

Example:

```yaml
os:
  modules:
  - name: br_netfilter
    loadMode: disable
  - name: vfio
    loadMode: always
```

## options [map\<string, string>]

Kernel options for modules can specify how these modules interact with the system,
and adjust performance or security settings specific to each module.

An error will be reported if module options are specified for a kernel module that has
been disabled in the image.

The options are specified in the `/etc/modprobe.d/module-options.conf` file.

Example:

```yaml
os:
  modules:
  - name: vfio
    loadMode: always
    options:
      enable_unsafe_noiommu_mode: Y
      disable_vga: Y
```
