---
parent: Configuration
ancestor: Image Customizer
---

# outputArtifacts type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-artifacts` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Specifies the configuration for the output directory containing the generated
artifacts, including UKI PE images, shim, bootloader (grub or systemd-boot), and verity
hash files.

After Image Customizer outputs the selected artifacts, it will also generate a helper
configuration file named `inject-files.yaml` under the same directory of output
artifacts. This file can later be used to inject signed artifacts back into an
image. For more details, see the [`injectFilesConfig`](./injectFilesConfig.md)
documentation.

The generated `inject-files.yaml` will include the `inject-files` preview feature.
COSI compression settings can be specified via CLI flags when running the
inject-files subcommand.

Example:

```yaml
output:
  artifacts:
    items: 
    - ukis
    - shim
    - bootloader
    - verity-hash
    path: ./output
previewFeatures:
- output-artifacts
```

Added in v0.14.

## path [string]

Required.

Specifies the directory path where Image Customizer will output the selected artifacts.

Added in v0.14.

## items [string[]]

Required.

Specifies the artifacts that will be output after the image customization.

Supported values:

- `ukis` – UKI PE images (`vmlinuz-<version>.efi`) and their associated addon files
  (`vmlinuz-<version>.efi.extra.d/vmlinuz-<version>.addon.efi`).
- `uki-addons` – UKI addon files only (`vmlinuz-<version>.addon.efi`), without
  the full UKI PE images.
- `shim` – Bootloader shim executable (`boot<arch>.efi`).
- `bootloader` – Bootloader executable (`grub<arch>.efi`).
  *Added in v1.3.*
- `verity-hash` – Verity hash files associated with dm-verity protected partitions.
  *Added in v0.16.*

The `output.artifacts` field must be used with the `output-artifacts` enabled in `previewFeatures`.

These artifacts are generated in an unsigned format and must be signed externally if required.

Supported architectures for shim and bootloader include x64 and arm64,
reflected in the `<arch>` portion of the filenames.

The `verity-hash` artifact will only be output if the corresponding Verity entry
defines a [`hashSignaturePath`](./verity.md#hashsignaturepath-string). If the
`hashSignaturePath` is not configured, Image Customizer will skip generating the
hash file for that Verity device. For more details, see the
[`verity`](./verity.md) documentation.

Added in v0.14.
