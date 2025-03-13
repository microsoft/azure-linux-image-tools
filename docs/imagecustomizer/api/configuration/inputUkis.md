---
parent: Configuration
---

# inputUkis type

Specifies the configuration for inputting signed UKI PE images into the image.

Example:

```yaml
ukis:
  path: /input/ukis
```

## path [string]

Required.

Specifies the directory path that contains all signed UKI PE images.

### Expected Files in the Directory

The specified directory must contain signed UKI PE images. Typical files found in this directory include:

- vmlinuz-<version>.signed.efi – A signed UKI PE image.

### Signed UKI PE Images

The files referenced in path should be pre-signed UKI PE images.

- Prism does not perform signing. It is recommended to use ESRP for signing UKI PE images.
- Supported format: .efi (signed UKI PE images).
