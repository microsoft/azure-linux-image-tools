---
parent: Configuration
ancestor: Image Customizer
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
    symlinkMode: preserve
```

Added in v0.3.

## source [string]

The absolute path to the source directory that will be copied.

Added in v0.3.

## destination [string]

The absolute path in the target OS that the source directory will be copied to.

Added in v0.3.

## symlinkMode [string]

Controls how symbolic links inside the source directory are copied.

Supported values:

- `dereference`: Follow symbolic links and copy their target contents. This is the
  default when `symlinkMode` is omitted, preserving the behavior of existing
  `additionalDirs` configurations.
- `preserve`: Recreate each symbolic link in the target OS with the same target string
  without reading the link target on the build host. This mode supports dangling links
  and links to special files. It requires the `preserve-symlinks`
  [preview feature](./config.md#previewfeatures-string) to be enabled.

The permission fields (`newDirPermissions`, `mergedDirPermissions`, and
`childFilePermissions`) are not applied to symbolic links when using `preserve`.

Added in v1.7.

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
