---
parent: Configuration
ancestor: Image Customizer
---

# signingMethodEphemeral type

Sign the artifacts with ephemeral keys. More specifically, the artifacts are signed with
a self-signed x509 certificate generated on the fly and whose private keys are deleted
after the signing is complete.

This signing method has the advantage of not needing to manage your own Certificate
Authority (CA). But it comes at the cost of needing to separately trust each OS image
that you build.

Added in v1.1.

## publicKeysPath [string]

When the ephemeral signing method is used, this specifies the directory to output the
public key files to.

If the path specified is relative, then it will be relative to the config file's parent
directory.

When ephemeral signing is used, either the `--ephemeral-public-keys-path` parameter or
the
[.signingMethod.ephemeral.publicKeysPath](../configuration/signingMethodEphemeral.md#publickeyspath-string)
field must be specified. If both are specified, then the `--ephemeral-public-keys-path`
parameter takes priority.

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
