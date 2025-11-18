---
parent: Configuration
ancestor: Image Customizer
---

# baseConfig type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `base-configs` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Defines a single configuration file to inherit from.

BaseConfigs specifies a list of base configuration files to inherit from.
When multiple base configs are specified, fields are resolved in order —
Fields from later configurations override or extend those from earlier ones,
or are processed sequentially.

**The current(last) config’s value (if specified) overrides all base configs.**

- `.input.image.path`  
- `.output.image.path`  
- `.output.image.format`  
- `.output.artifacts.path`
- `.os.hostname`
- `.os.selinux`
- `.os.uki`

**Base config items are merged with current config’s items**

- `.output.artifacts.items`
- `.os.kernelCommandLine`

**Base config items are processed first, followed by current config’s.**

- `.os.users`
- `.os.groups`
- `.os.services`
- `.os.packages` (If .os.packages.snapshotTime is specified, it is applied per-config)
- `.os.modules`
- `.os.additionalFiles`
- `.os.additionalDirs`
- `.os.overlays`

**Pick the strongest option from the base config and current config.**

- `.os.bootLoader.resetType`

  The option ordering, from strongest to weakest:
  - `"hard-reset"`
  - `""` (i.e. no reset)

**Storage Override Fields:**

Current value (if specified) fully overrides base's value
- `.storage.disks`
- `.storage.verity`
- `.storage.bootType`
- `.storage.filesystems`

**Storage configuration also follows specific rules based on field combinations:**

| Base\Current | None | `.storage.disks` | `.storage.resetPartitionsUuidsType` | `.storage.reinitializeVerity` |
|--------------|------|------------------|-------------------------------------|------------------------------|
| **None** | None | Current's `.storage.disks` | Current's `.storage.resetPartitionsUuidsType` | Current's `.storage.reinitializeVerity` |
| **`.storage.disks`** | Base's `.storage.disks` | Current's `.storage.disks` | Error¹ | Error² |
| **`.resetPartitionsUuidsType`** | Base's `.storage.resetPartitionsUuidsType` | Current's `.storage.disks` | Current's `.storage.resetPartitionsUuidsType` | Base's reset + Current's reinit³ |
| **`.storage.reinitializeVerity`** | Base's `.storage.reinitializeVerity` | Current's `.storage.disks` | Base's reinit + Current's reset³ | Current's `.storage.reinitializeVerity` |

¹ The current config requesting the partition UUIDs to be reset when the base config customized the partition table is somewhat pointless. Hence we error out.

² Cannot reinitialize verity when base config destroys original verity setup with partition layout changes.

³ Base's `.resetPartitionsUuidsType` and current's `.reinitializeVerity` are compatible and both are preserved together since UUID reset updates partition identifiers while verity reinit updates verity mapping to use new identifiers and rebuilds hash trees.

## path [string]

Required.

A file path to the base config file. The path can be either relative or absolute.
Relative paths are resolved relative to the parent directory of the current config file.

Example:

```yaml
baseConfigs:
  - path: ./base-config.yaml
  - path: /absolute/path/to/base-config.yaml
```

Added in v1.1.0.
