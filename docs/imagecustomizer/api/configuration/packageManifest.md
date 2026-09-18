---
parent: Configuration
ancestor: Image Customizer
---

# packageManifest type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `package-manifest` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Enables the management of the image's package manifest. On images
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

- `create`: Write a manifest after the configured package operations,
  `postCustomization` scripts, and any requested removal of the package manager
  have run. Any existing manifest is overwritten, and one is created
  if none exists.

- `passthrough`: Do not read, validate, or modify the manifest. Package changes
  can leave it stale, and the caller takes responsibility for that. This mode
  does not create a missing manifest.

- `none`: Remove the manifest if one exists.
  This mode cannot be combined with
  [output.packageManifest.path](./outputPackageManifest.md#path-string) or
  [--output-package-manifest-file](../cli/customize.md#--output-package-manifest-filefile-path).

There is no default mode. If the base image has a manifest, removing the
package manager is requested, or a manifest output path is specified,
an explicit mode is required. Otherwise, `os.packages.manifest` may be omitted,
in which case Image Customizer will not do anything.

Creating or deleting the manifest requires its directory to be writable. On an
image with a verity-protected `/usr`, set
[storage.reinitializeVerity: all](./storage.md#reinitializeverity-string) and
enable the `reinitialize-verity` preview feature before changing the manifest.

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
