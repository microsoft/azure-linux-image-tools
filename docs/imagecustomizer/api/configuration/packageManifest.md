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

### Generated manifest

Under `create`, the manifest is rebuilt using the current build timestamp,
even if the package set is unchanged.

The document `name` is the distribution's identifier.
Its `documentNamespace` is deterministic for the document `name`, the root package's
`versionInfo`, and the installed package NEVRAs sorted in ascending order.

The root package's `name` is the distribution's identifier, its `supplier` is
always `NOASSERTION`, and its `versionInfo` is the base image's `VERSION`, with
`+<BUILD_ID>` appended if `BUILD_ID` is specified. These values are captured during
target OS detection from `/etc/os-release`, falling back to `/usr/lib/os-release`
if it is absent.

Each package's `supplier` is `Organization: <RPM vendor>` or
`NOASSERTION` if the RPM has no vendor specified.

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
