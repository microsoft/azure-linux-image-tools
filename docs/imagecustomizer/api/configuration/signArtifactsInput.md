---
parent: Configuration
ancestor: Image Customizer
---

# signArtifactsInput type

Specifies the input parameters for the [sign-artifacts](../cli/sign-artifacts.md)
subcommand.

Added in v1.1.

## artifactsPath [string]

The path of the directory containing the artifacts to sign.

This should be the same directory that was specified in the
[.output.artifacts.path](../configuration/outputArtifacts.md#path-string) field.

If the path specified is relative, then it will be relative to the config file's parent
directory.

Example:

```yaml
previewFeatures:
- sign-artifacts

input:
  artifactsPath: ./out/artfiacts
  
signingMethod:
  ephemeral:
    publicKeysPath: ./out/public-keys
```

Added in v1.1.
