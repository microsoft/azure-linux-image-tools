---
parent: Configuration
ancestor: Image Customizer
---

# packages type

Specifies packages to remove, install, or update.

## Azure Linux

Packages are managed using `tdnf`.

Package names can be specified in the following formats:

- `<package-name>`
- `<package-name>.<arch>`
- `<package-name>-<version>`
- `<package-name>-<version>-<release>.<distro>`
- `<package-name>-<version>-<release>.<distro>.<arch>`

Note: Package names like to `parted-3.4-2` will not work. You must include the distro
tag. For example, `parted-3.4-2.cm2` will work. (`cm2` means CBL-Mariner 2.0.)

## Ubuntu

Packages are managed using `apt-get`.

Package names can be specified in the following formats:

- `<package-name>` (e.g. `openssh-server`)
- `<package-name>=<version>` (e.g. `openssh-server=1:8.9p1-3ubuntu0.13`)

Note: When specifying package versions, only the full version is recognized.
Partial versions (e.g. `openssh-server=1:8.9p1`) will not work.

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

Implemented by calling: `tdnf install` (Azure Linux) or `apt-get install` (Ubuntu).

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

## snapshotTime [string]

Filters packages by publication time.

Only packages published before the specified timestamp will be considered during
install or update. This supports both ISO 8601 date-only format (`YYYY-MM-DD`)
and full RFC 3339 timestamp (`YYYY-MM-DDTHH:MM:SSZ`).

If this value is specified, then a `package-snapshot-time` entry must be added to
[previewFeatures](./config.md#previewfeatures-string)

Example:

```yaml
previewFeatures:
- package-snapshot-time

os:
  packages:
    snapshotTime: 2025-05-20T23:59:59Z
```

Added in v0.15.

## removePackageManager [bool]

Optional.

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `remove-package-manager` in the
[previewFeatures](./config.md#previewfeatures-string) API.

When set to true, the package manager tooling (e.g. tdnf, dnf, apt, etc.) and all the
package management files (e.g. databases, package cache) will be removed from the image.

Note: For Ubuntu, the dpkg package and its related files are not removed since it is a
dependency of some fundamental system packages (e.g. `grub-efi-amd64-signed`).

This operation includes removing any packages that are considered "unused dependencies"
by the package manager. If this removes a package required in your image, then mark the
required package in a [postCustomization](./scripts.md#postcustomization-script) script
to prevent this:

- Azure Linux 3: `tdnf mark install <package>`
- Azure Linux 4 / Fedora: `dnf mark install <package>`
- Ubuntu: `apt-mark install <package>`

This operation occurs after the `postCustomization` scripts and before the
`finalizeCustomization` scripts. See,
[Operation Ordering](./configuration.md#operation-ordering) for details.

Note: If this API is used when the
[output format](../cli/customize.md#--output-image-formatformat)
is either `iso`, `pxe-dir`, `pxe-tar`, `cosi`, or `baremetal-image`, then the build will
fail. This is planned to be fixed in a future release.

Example:

```yaml
previewFeatures:
- remove-package-manager

os:
  packages:
    removePackageManager: true
```

Added in v1.6.
