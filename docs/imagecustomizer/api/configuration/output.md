---
parent: Configuration
---

# output type

Specifies the configuration for the output image and artifacts.

Example:

```yaml
output:
  image:
    path: ./out/image.vhdx
    format: vhdx
  artifacts:
    items: 
    - ukis
    - shim
    - systemdBoot
    path: /home/usr/output
previewFeatures:
- output.artifacts
```

## image [[outputImage](./outputImage.md)]

Specifies the configuration for the output image.

Added in v0.13.0.

## artifacts [[outputArtifacts](./outputArtifacts.md)]

Specifies the configuration for the output directory containing the generated artifacts.
