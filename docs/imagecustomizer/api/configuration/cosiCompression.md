---
parent: Configuration
ancestor: Image Customizer
---

# cosiCompression type

Specifies the zstd compression settings for the partition images within `cosi` and
`baremetal-image` output images.

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
```

## level [int]

Optional. Default: `9` for `cosi`, `22` for `baremetal-image`.

If both `output.image.cosi.compression.level` and
[--cosi-compression-level](../cli/customize.md#--cosi-compression-levellevel) are provided, then the
`--cosi-compression-level` value is used.

The zstd compression level (1-22) for COSI partition images.

Higher compression levels produce smaller files but take significantly longer to
compress. Decompression speed is largely unaffected by the compression level.

Added in v1.2.
