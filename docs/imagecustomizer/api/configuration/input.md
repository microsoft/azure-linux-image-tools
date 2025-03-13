---
parent: Configuration
---

# input type

Specifies the configuration for the input image and associated files required for customizing.

Example:

```yaml
input:
  image:
    path: ./base.vhdx
  verityHashes:
    path: /input/verityhashes
  ukis:
    path: /input/ukis
  bootloader:
    path: /input/bootloader
```

## image [[inputImage](./inputImage.md)]

Specifies the configuration for the input image.

Added in v0.13.0.

## verityHashes [[inputVerityHashes](./inputVerityHashes.md)]

Specifies the configuration for inputting signed Verity hash files into the image.

## ukis [[inputUkis](./inputUkis.md)]

Specifies the configuration for inputting signed UKI PE images into the image.

## bootloader [[inputBootloader](./inputBootloader.md)]

Specifies the configuration for inputting signed bootloader components, including shim and systemd-boot, into the image.
