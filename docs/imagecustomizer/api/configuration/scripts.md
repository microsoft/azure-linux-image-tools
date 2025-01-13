# scripts type

Specifies custom scripts to run during the customization process.

Note: Script files must be in the same directory or a child directory of the directory
that contains the config file.

## postCustomization [[script](./script.md)[]]

Scripts to run after all the in-built customization steps have run.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  postCustomization:
  - path: scripts/a.sh
```

## finalizeCustomization [[script](./script.md)[]]

Scripts to run at the end of the customization process.

In particular, these scripts run after:

1. The `setfiles` command has been called to update/fix the SELinux files labels (if
   SELinux is enabled), and

2. The temporary `/etc/resolv.conf` file has been deleted,

but before the conversion to the requested output type.
(See, [Operation ordering](../configuration.md#operation-ordering) for details.)

Most scripts should be added to [postCustomization](#postcustomization-script).
Only add scripts to [finalizeCustomization](#finalizecustomization-script) if you want
to customize the `/etc/resolv.conf` file or you want manually set SELinux file labels.

These scripts are run under a chroot of the customized OS.

Example:

```yaml
scripts:
  finalizeCustomization:
  - path: scripts/b.sh
```
