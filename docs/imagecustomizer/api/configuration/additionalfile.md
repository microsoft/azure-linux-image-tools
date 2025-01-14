---
parent: Configuration
---

# additionalFile type

Specifies options for placing a file in the OS.

Type is used by: [additionalFiles](./config.md#additionalfiles-additionalfile)

## source [string]

The path of the source file to copy to the destination path.

Example:

```yaml
os:
  additionalFiles:
    files/a.txt:
    - path: /a.txt
```

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

## destination [string]

The absolute path of the destination file.

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
```

## permissions [string]

The permissions to set on the destination file.

Supported formats:

- String containing an octal string. e.g. `"664"`

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
    permissions: "664"
```
