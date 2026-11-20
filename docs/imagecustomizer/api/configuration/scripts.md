---
parent: Configuration
ancestor: Image Customizer
---

# scripts type

Specifies custom scripts to run during the customization process.

Note: Script files must be in the same directory or a child directory of the directory
that contains the config file.

Added in v0.3.

## postCustomization [[script](./script.md)[]]

Scripts to run after the main in-built customization steps, before package-manager
removal and package manifest creation.

Package changes made by these scripts are included when
[os.packages.manifest.mode](./packageManifest.md#mode-string) is `create`.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
```

Added in v0.3.

## finalizeCustomization [[script](./script.md)[]]

Scripts to run at the end of the customization process.

In particular, these scripts run after:

1. The `setfiles` command has been called to update/fix the SELinux files labels (if
   SELinux is enabled), and

2. The temporary `/etc/resolv.conf` file has been deleted,

3. The package manager has been removed (if specified)

4. The package manifest at `/usr/share/os-manifests/package-manifest.spdx.json` has
  been created, preserved, or removed according to
  [os.packages.manifest.mode](./packageManifest.md#mode-string), if specified

but before the conversion to the requested output type.
(See, [Operation ordering](./configuration.md#operation-ordering) for details.)

Most scripts should be added to [postCustomization](#postcustomization-script).
Only add scripts to [finalizeCustomization](#finalizecustomization-script) if you want
to customize the `/etc/resolv.conf` or `/usr/share/os-manifests/package-manifest.spdx.json` files,
or manually set SELinux file labels.

Package management operations in particular must never be used in a finalize script,
since by that time the package manager may have been removed, and any package
changes made at that stage are not reflected automatically in the package manifest.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  finalizeCustomization:
  - path: scripts/b.sh
```

Added in v0.3.
