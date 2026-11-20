---
parent: Configuration
ancestor: Image Customizer
---

# additionalFile type

Specifies options for placing a file in the OS.

Type is used by: [additionalFiles](./os.md#additionalfiles-additionalfile)

Added in v0.7.

## source [string]

The path of the source file to copy to the destination path.

Example:

```yaml
os:
  additionalFiles:
    files/a.txt:
    - path: /a.txt
```

Added in v0.7.

## content [string]

The contents of the file to write to the destination path.

Example:

```yaml
os:
  additionalFiles:
  - content: |
      abc
    destination: /a.txt
```

Added in v0.7.

## destination [string]

The absolute path of the destination file.

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
```

Added in v0.7.

## permissions [string]

The permissions to set on the destination file.

Supported formats:

- String containing an octal string (e.g. `"664"`)

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
    permissions: "664"
```

Added in v0.7.

## symlinkMode [string]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `symlink-mode` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Controls how a symbolic link given as `source` is copied. Only applies when `source` is
set; setting `symlinkMode` without `source` is rejected during validation.

When `symlinkMode` is omitted, the link is dereferenced, matching the behavior of existing
`additionalFiles` configurations.

Supported values:

- `dereference`: Follow the symbolic link and copy its target contents. This is also the
  behavior when `symlinkMode` is omitted.
- `preserve`: Recreate the symbolic link at the destination with the same target string
  without reading the link target on the build host. This supports dangling links and
  links to special files.

The `permissions` field is not applied to the symbolic link itself when using `preserve`.

Added in v1.7.
