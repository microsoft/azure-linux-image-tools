---
parent: Configuration
ancestor: Image Customizer
---

# cosiCompression type

Specifies the zstd compression settings for the partition images within the COSI output image.

If not specified, the default compression level is used.

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

## level [int]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `cosi-compression` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Optional. Default: `9`

If both `output.image.cosi.compression.level` and
[--cosi-compression-level](../cli/cli.md#--cosi-compression-levellevel) are provided, then the
`--cosi-compression-level` value is used.

The zstd compression level (1-22) for COSI partition images.

Higher compression levels produce smaller files but take significantly longer to
compress. Decompression speed is largely unaffected by the compression level.

Added in v1.2.
