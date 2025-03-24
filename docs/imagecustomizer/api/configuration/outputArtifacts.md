---
parent: Configuration
---

# outputArtifacts type

Specifies the configuration for the output directory containing the generated
artifacts, including UKI PE images, shim and systemd-boot.

Example:

```yaml
output:
  artifacts:
    items: 
    - ukis
    - shim
    - systemd-boot
    path: ./output
previewFeatures:
- output-artifacts
```

## path [string]

Required.

Specifies the directory path where Prism will output the selected artifacts.

## items [string[]]

Required.

Specifies the artifacts that will be selected to output after the image customization.

Supported values:

- `ukis` – UKI PE images (`vmlinuz-<version>.efi`).
- `shim` – Bootloader shim executable (`boot<arch>.efi`).
- `systemd-boot` – Systemd-boot executable (`systemd-boot<arch>.efi`).

The `output-artifacts` field must be used with the `output-artifacts` enabled in `previewFeatures`.

These artifacts are generated in an unsigned format and must be signed externally if required.

Supported architectures for shim and systemd-boot include x64 and arm64, reflected in the <arch> portion of the filenames.
