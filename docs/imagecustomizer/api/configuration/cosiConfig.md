---
parent: Configuration
ancestor: Image Customizer
---

# cosiConfig type

Specifies the configuration options for the `cosi` and `baremetal-image` output
formats.

Example:

```yaml
output:
  image:
    path: ./out/image.cosi
    format: cosi
    cosi:
      compression:
        level: 22
```

## compression [[cosiCompression](./cosiCompression.md)]

Optional.

Specifies the compression settings for the partition images within the COSI output image.

Added in v1.2.
