---
title: sign-artifacts
parent: Command line
ancestor: Image Customizer
nav_order: 2
---

# sign-artifacts subcommand

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `sign-artifacts` in the
[previewFeatures](../configuration/signArtifactsConfig.md#previewfeatures-string) API.

This subcommand takes the directory containing files populated by the
[.output.artifacts](../configuration/output.md#artifacts-outputartifacts) API and signs
the files.

The typical flow for this API is:

1. Customize the image and output artifacts that need to be signed using the
  [.output.artifacts](../configuration/output.md#artifacts-outputartifacts) API.

2. Call the `sign-artifacts` API to sign the artifacts.

3. Call the `inject-files` API to inject the signed artifacts back into the image.

Added in v1.1.

## --build-dir

A directory where temporary files can be placed.

Required.

Added in v1.1.

## --config-file

A path to the configuration file that specifies how to sign the artifacts.

Required.

For documentation on the supported configuration options, see:
[signArtifactsConfig type](../configuration/signArtifactsConfig.md)

Added in v1.1.

## --artifacts-paths

The path of the directory containing the artifacts to sign.

This should be the same directory that was specified in the
[.output.artifacts.path](../configuration/outputArtifacts.md#path-string) field.

If the path specified is relative, then it will be relative to the current working
directory.

Either the `--artifacts-paths` parameter or the
[.input.artifactsPath](../configuration/signArtifactsInput.md#artifactspath) field must
be specified. If both are specified, the `--artifacts-paths` parameter takes priority.

Added in v1.1.

## --ephemeral-public-keys-path

When the ephemeral signing method is used, this specifies the directory to output the
public key files to.

If the path specified is relative, then it will be relative to the current working
directory.

When ephemeral signing is used, either the `--ephemeral-public-keys-path` parameter or
the
[.signingMethod.ephemeral.publicKeysPath](../configuration/signingMethodEphemeral.md#publickeyspath-string)
field must be specified. If both are specified, then the `--ephemeral-public-keys-path`
parameter takes priority.

Added in v1.1.
