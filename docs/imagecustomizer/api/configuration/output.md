---
parent: Configuration
---

# output type

Specifies the configuration for the output image and associated files required for customizing.

Example:

```yaml
output:
  image:
    path: ./out/image.vhdx
    format: vhdx
  verityHashes:
    path: /output/verityhashes
  ukis:
    path: /output/ukis
  bootloader:
    path: /output/bootloader
```

## image [[outputImage](./outputImage.md)]

Specifies the configuration for the output image.

Added in v0.13.0.

## verityHashes [[outputVerityHashes](./outputVerityHashes.md)]

Specifies the configuration for the output directory containing the generated Verity hash files.

## ukis [[outputUkis](./outputUkis.md)]

Specifies the configuration for the output directory containing the generated UKI PE images.

## bootloader [[outputBootloader](./outputBootloader.md)]

Specifies the configuration for the output directory containing the generated bootloader components, including shim and systemd-boot.
