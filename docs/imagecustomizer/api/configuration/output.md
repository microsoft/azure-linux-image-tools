---
parent: Configuration
---

# output type

Specifies the configuration for the output image.

Example:

```yaml
output:
  path: ./out/image.vhdx
```

## path [string]

Required, unless
[--output-image-file](../cli.md#--output-image-filefile-path) is provided
on the command line. If both `--output-image-file` are `output.path` are
provided, then the value of `--output-image-file` is used.

The file path to write the final customized image to.

Added in v0.13.0.
