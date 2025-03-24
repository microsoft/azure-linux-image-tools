---
parent: Configuration
---

# output type

Specifies the configuration for the output image and artifacts.

## image [[outputImage](./outputImage.md)]

Specifies the configuration for the output image.

Example:

```yaml
output:
  image:
    path: ./out/image.vhdx
    format: vhdx
```

Added in v0.13.0.

## artifacts [[outputArtifacts](./outputArtifacts.md)]

Specifies the configuration for the output directory containing the generated artifacts.

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
