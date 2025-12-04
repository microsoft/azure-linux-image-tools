---
parent: Configuration
ancestor: Image Customizer
---

# signArtifactsConfig type

The configuration API for the [sign-artifacts](../cli/sign-artifacts.md) subcommand.

Added in v1.1.

## previewFeatures [string[]]

Enables preview features.

Preview features are features that have not yet been stabilized.
Their APIs and behavior are subject to change.

Supported options:

- `sign-artifacts`: Enables the use of the [sign-artifacts](../cli/sign-artifacts.md)
  subcommand.

  Added in v1.1.

## input [[signArtifactsInput](./signArtifactsInput.md)]

Specifies the input parameters.

Added in v1.1.

## signingMethod [[signingMethod](./signingMethod.md)]

Specifies the method to use to sign the artifacts and the options for that signing
method.

Added in v1.1.
