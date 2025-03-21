---
parent: Configuration
---

# outputBootloader type

Specifies the configuration for the output directory containing the generated bootloader components, including shim and systemd-boot.

Example:

```yaml
bootloader:
  path: /output/bootloader
```

## path [string]

Required.

Specifies the directory path where Prism will output the generated bootloader components.

### Expected Files in the Directory

After the image customization process, this directory will contain the following unsigned bootloader components:

- bootx64.efi – The generated shim executable.
- systemd-bootx64.efi – The generated systemd-boot executable.

### Generated Bootloader Components

- These files are not signed—Prism only generates them.
- Signing must be performed externally using a signing service such as ESRP.
- Required components:
  - Unsigned shim executable (bootx64.efi)
  - Unsigned systemd-boot (systemd-bootx64.efi)
- Supported format: .efi (unsigned bootloader executables).
