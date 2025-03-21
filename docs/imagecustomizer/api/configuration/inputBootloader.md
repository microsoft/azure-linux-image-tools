---
parent: Configuration
---

# inputBootloader type

Specifies the configuration for inputting signed bootloader components, including shim and systemd-boot, into the image.

Example:

```yaml
bootloader:
  path: /input/bootloader
```

## path [string]

Required.

Specifies the directory path that contains all required signed bootloader components.

### Bootloader Requirements

The specified directory must contain the following pre-signed bootloader components:

- bootx64.efi – The signed shim executable.
- systemd-bootx64.efi – The signed systemd-boot executable.

### Signed Bootloader Components

The files in path should be pre-signed bootloader executables.

- Prism does not perform signing. It is recommended to use ESRP for signing bootloader components.
- Required components:
  - Signed shim executable (bootx64.efi)
  - Signed systemd-boot (systemd-bootx64.efi)
- Supported format: .efi (signed bootloader executables).
