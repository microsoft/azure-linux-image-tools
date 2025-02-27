---
parent: Configuration
---

# dirConfig type

Specifies options for placing a directory in the OS.

Type is used by: [additionalDirs](./os.md#additionaldirs-dirconfig)

Example:

```yaml
os:
  additionalDirs:
  - source: "home/files/targetDir"
    destination: "usr/project/targetDir"
```

Added in v0.3.

## source [string]

The absolute path to the source directory that will be copied.

Added in v0.3.

## destination [string]

The absolute path in the target OS that the source directory will be copied to.

Added in v0.3.

## newDirPermissions [string]

The permissions to set on all of the new directories being created on the target OS
(including the top-level directory). Default value: `755`.

Added in v0.3.

## mergedDirPermissions [string]

The permissions to set on the directories being copied that already do exist on the
target OS (including the top-level directory). **Note:** If this value is not specified
in the config, the permissions for this field will be the same as that of the
pre-existing directory.

Added in v0.3.

## childFilePermissions [string]

The permissions to set on the children file of the directory. Default value: `755`.

Supported formats for permission values:

- String containing an octal value. e.g. `664`

Example:

```yaml
os:
  additionalDirs:
  - source: "home/files/targetDir"
    destination: "usr/project/targetDir"
    newDirPermissions: "644"
    mergedDirPermissions: "777"
    childFilePermissions: "644"
```

Added in v0.3.
