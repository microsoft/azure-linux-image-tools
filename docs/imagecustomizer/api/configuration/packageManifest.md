---
parent: Configuration
ancestor: Image Customizer
---

# packageManifest type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `package-manifest` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Enables the management of the customized image's package manifest. On images
without a package manager, vulnerability scanners rely on this manifest to identify
installed packages. Shipping such an image with a stale or missing manifest can
cause scanners to miss vulnerabilities and is therefore not recommended.

Example:

```yaml
os:
  packages:
    install:
    - vim
    manifest:
      mode: create
previewFeatures:
- package-manifest
```

Added in v1.7.

## mode [string]

Required.

Specifies how to manage the SPDX 2.2 package manifest at
`/usr/share/os-manifests/package-manifest.spdx.json`.

Supported values:

- `create`: Write a manifest describing the packages the image has once every
  package operation has run, including the removal of the package manager.
  Any manifest the base image carries is written over, and one is created if it
  has none.

- `passthrough`: Leave the base image's manifest exactly as it was.
  Any package operation leaves it stale, and the caller takes responsibility for
  that. It is not created if the base image does not have one.

- `none`: Remove the manifest if the base image has one.
  This mode cannot be combined with
  [output.packageManifest.path](./outputPackageManifest.md#path-string) or
  [--output-package-manifest-file](../cli/customize.md#--output-package-manifest-filefile-path).

There is no default mode. If the base image has a manifest, removing the
package manager is requested, or a manifest output path is specified,
an explicit mode is required. Otherwise, `os.packages.manifest` may be omitted,
in which case Image Customizer will not do anything.

Example:

```yaml
os:
  packages:
    manifest:
      mode: create
previewFeatures:
- package-manifest
```

Added in v1.7.
