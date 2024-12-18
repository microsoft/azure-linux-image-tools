# module type

Options for configuring a kernel module.

## name [string]

Name of the module.

```yaml
os:
  modules:
  - name: br_netfilter
```

## loadMode [string]

The loadMode setting for kernel modules dictates how and when these modules 
are loaded or disabled in the system.

Supported values:

- `always`: Set kernel modules to be loaded automatically at boot time.
  - If the module is blacklisted in the base image, remove the blacklist entry.
  - Add the module to `/etc/modules-load.d/modules-load.conf`.
  - Write the options, if provided.

- `auto`: Used for modules that are automatically loaded by the kernel as needed,
    without explicit configuration to load them at boot.
  - If the module is disabled in the base image, remove the blacklist entry to
    allow it to be loaded automatically.
  - Write the provided options to `/etc/modprobe.d/module-options.conf`, but do not
    add the module to `/etc/modules-load.d/modules-load.conf`, as it should be loaded automatically by
    the kernel when necessary.

- `disable`: Configures kernel modules to be explicitly disabled, preventing them from
  loading automatically.
  - If the module is not already disabled in the base image, a blacklist entry will
    be added to `/etc/modprobe.d/blacklist.conf` to ensure the module is disabled.

- `inherit`: Configures kernel modules to inherit the loading behavior set in the base
  image. Only applying new options where they are explicitly provided and applicable.
  - If the module is not disabled, and options are provided, these options will be
    written to `/etc/modprobe.d/module-options.conf`.

- empty string or not set, it will default to `inherit`.

## options [map\<string, string>]

Kernel options for modules can specify how these modules interact with the system, 
and adjust performance or security settings specific to each module.

```yaml
os:
  modules:
  - name: vfio
    loadMode: always
    options:
      enable_unsafe_noiommu_mode: Y
      disable_vga: Y
```
