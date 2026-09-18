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

Controls how a symbolic link given as `source` is copied. Only applies when `source` is
set (not `content`).

Supported values:

- `dereference`: Follow the symbolic link and copy its target contents. This is the
  default when `symlinkMode` is omitted, preserving the behavior of existing
  `additionalFiles` configurations.
- `preserve`: Recreate the symbolic link at the destination with the same target string
  without reading the link target on the build host. This supports dangling links and
  links to special files. It requires the `preserve-symlinks`
  [preview feature](./config.md#previewfeatures-string) to be enabled, and cannot be used
  with `content`.

The `permissions` field is not applied to the symbolic link itself when using `preserve`.

Added in v1.7.
