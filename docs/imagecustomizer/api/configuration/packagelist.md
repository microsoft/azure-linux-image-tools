# packageList type

Used to split off lists of packages into a separate file.
This is useful for sharing list of packages between different configuration files.

This type is used by:

- [installLists](./packages.md#installlists-string)
- [removeLists](./packages.md#removelists-string)
- [updateLists](./packages.md#updatelists-string)

## packages [string[]]

Specifies a list of packages.

Example:

```yaml
packages:
- openssh-server
```