---
parent: Configuration
---

# outputArtifacts type

Specifies the configuration for the output directory containing the generated
artifacts, including UKI PE images, shim and systemd-boot.

Example:

```yaml
artifacts:
  items: 
  - ukis
  - shim
  - systemdBoot
  path: /home/usr/output
```

## path [string]

Required.

Specifies the directory path where Prism will output the selected artifacts.

## items [string[]]

Required.

Specifies the artifacts that will be selected to output after the image customization.

### Supported Artifacts

- `ukis` – UKI PE images (`vmlinuz-<version>.efi`).
- `shim` – Bootloader shim executable (`boot<arch>.efi`).
- `systemdBoot` – Systemd-boot executable (`systemd-boot<arch>.efi`).

### Additional Requirements

The `output.artifacts` field must be used with the `output.artifacts` enabled in `PreviewFeatures`.

These artifacts are generated in an unsigned format and must be signed externally if required.

Supported architectures for shim and systemd-boot include x64 and arm64, reflected in the <arch> portion of the filenames.
