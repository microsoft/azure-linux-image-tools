---
parent: Configuration
---

# packages type

Specifies packages to remove, install, or update.

Package names can be specified in the following formats:

- `<package-name>`
- `<package-name>.<arch>`
- `<package-name>-<version>`
- `<package-name>-<version>-<release>.<distro>`
- `<package-name>-<version>-<release>.<distro>.<arch>`

Note: Package names like to `parted-3.4-2` will not work. You must include the distro
tag. For example, `parted-3.4-2.cm2` will work. (`cm2` means CBL-Mariner 2.0.)

Added in v0.3.

## updateExistingPackages [bool]

Updates the packages that exist in the base image.

Implemented by calling: `tdnf update`

Example:

```yaml
os:
  packages:
    updateExistingPackages: true
```

Added in v0.3.

## installLists [string[]]

Same as [install](#install-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](./packagelist.md).

Example:

```yaml
os:
  packages:
    installLists:
    - lists/ssh.yaml
```

Added in v0.3.

## install [string[]]

Installs packages onto the image.

Implemented by calling: `tdnf install`.

Example:

```yaml
os:
  packages:
    install:
    - openssh-server
```

Added in v0.3.

## removeLists [string[]]

Same as [remove](#remove-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](./packagelist.md).

Example:

```yaml
os:
  packages:
    removeLists:
    - lists/ssh.yaml
```

Added in v0.3.

## remove [string[]]

Removes packages from the image.

Implemented by calling: `tdnf remove`

Example:

```yaml
os:
  packages:
    remove:
    - openssh-server
```

Added in v0.3.

## updateLists [string[]]

Same as [update](#update-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](./packagelist.md).

Example:

```yaml
os:
  packages:
    updateLists:
    - lists/ssh.yaml
```

Added in v0.3.

## update [string[]]

Updates packages on the system.

Implemented by calling: `tdnf update`

Example:

```yaml
os:
  packages:
    update:
    - openssh-server
```

Added in v0.3.
