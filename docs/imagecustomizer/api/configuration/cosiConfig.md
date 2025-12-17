---
parent: Configuration
ancestor: Image Customizer
---

# cosiConfig type

Specifies the configuration options for the COSI output format.

Example:

```yaml
output:
  image:
    path: ./out/image.cosi
    format: cosi
    cosi:
      compression:
        level: 22

previewFeatures:
- cosi-compression
```

## compression [[cosiCompression](./cosiCompression.md)]

Optional.

Specifies the compression settings for the partition images within the COSI output image.

Added in v1.2.
