---
parent: Configuration
ancestor: Image Customizer
---

# outputPackageManifest type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `package-manifest` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Specifies the configuration for the package manifest output file.

Example:

```yaml
os:
  packages:
    manifest:
      mode: create
output:
  packageManifest:
    path: ./out/package-manifest.spdx.json
previewFeatures:
- package-manifest
```

## path [string]

Required.

The path to write the image's package manifest to. It is copied from
`/usr/share/os-manifests/package-manifest.spdx.json` after
[os.packages.manifest.mode](../configuration/packageManifest.md#mode-string) has been
applied.

If specified, [os.packages.manifest.mode](./packageManifest.md#mode-string)
is required. The mode cannot be `none`, which leaves no manifest to copy.
If the base image has no package manifest, the mode must be `create`.

If both [--output-package-manifest-file](../cli/customize.md#--output-package-manifest-filefile-path)
and `output.packageManifest.path` are specified, then the value of
`--output-package-manifest-file` is used.

Added in v1.7.
