---
parent: Configuration
---

# group type

Options for configuring a user group.

Added in v0.16.

## name [string]

Required.

The name of the group.

Example:

```yaml
os:
  groups:
  - name: test
```

Added in v0.16.

## gid [int]

The ID to use for the group.

If the group already exists, providing this value will result in an error.

Valid range: 0-60000

Example:

```yaml
os:
  groups:
  - name: test
    gid: 1000
```

Added in v0.16.
