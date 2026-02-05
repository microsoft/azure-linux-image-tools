---
title: validate-config
parent: Command line
ancestor: Image Customizer
nav_order: 5
---

# validate-config subcommand

This subcommand validates configuration files for the [customize subcommand](./customize.md)
without running the actual customization process. For that reason, it does not require root
permissions and can be executed by non-privileged users.

Added in v1.2.

## --config-file=FILE-PATH

Required.

The file path of the YAML (or JSON) configuration file that specifies how an image should be customized.

For documentation on the supported configuration options, see:
[Image Customizer configuration](../configuration/configuration.md)

Added in v1.2.

## --build-dir=DIRECTORY-PATH

Optional. Required when `--validate-resources` includes `oci` or `all`.

The directory where the tool will place its temporary files, if required.

Added in v1.2.

## --validate-resources=RESOURCE[,RESOURCE...]

Optional.

Can be specified multiple times or as a comma-separated list.

Specifies which resources referenced in the configuration should be validated for existence.
Without this flag, only the configuration syntax and structure are validated.

Supported options:

- `files`: Validates that local files and directories exist:
  - [`iso.additionalFiles[].source`](../configuration/iso.md#additionalfiles-additionalfile): Must point to
    existing files.
  - [`os.additionalDirs[].source`](../configuration/os.md#additionaldirs-dirconfig): Must point to existing
    directories.
  - [`os.additionalFiles[].source`](../configuration/os.md#additionalfiles-additionalfile): Must point to
    existing files.
  - [`os.users[].password.value`](../configuration/password.md#value-string): Must point to an existing file
    when [`.type`](../configuration/password.md#type-string) is `plain-text-file` or `hashed-file`.
  - [`os.users[].sshPublicKeyPaths[]`](../configuration/user.md#sshpublickeypaths-string): Must point to
    existing files.
  - [`pxe.additionalFiles[].source`](../configuration/pxe.md#additionalfiles-additionalfile): Must point to
    existing files.
  - [`scripts.finalizeCustomization[].path`](../configuration/script.md#path-string): Must point to existing
    files.
  - [`scripts.postCustomization[].path`](../configuration/script.md#path-string): Must point to existing files.

- `oci`: Validates that OCI artifacts exist:
  - [`input.image.azureLinux`](../configuration/inputImage.md#azurelinux-azurelinuximage): OCI artifact must
    exist in Microsoft Artifact Registry with a valid notary signature.
  - [`input.image.oci.uri`](../configuration/ociimage.md#uri-string): OCI artifact must exist.

- `all`: Validates all supported resource types. Currently equivalent to `files,oci`.

  The meaning of `all` may expand in future versions as new resource types are added. Use `all` when you want
  comprehensive validation of all supported resources, or specify individual resource types explicitly for more
  predictable behavior across versions.

Added in v1.2.

## Examples

Validate configuration syntax only:

```bash
imagecustomizer validate-config --config-file=./config.yaml
```

Validate configuration syntax and local files:

```bash
imagecustomizer validate-config --config-file=./config.yaml --validate-resources=files
```

Validate configuration syntax, local files, and OCI artifacts using a comma-separated list:

```bash
imagecustomizer validate-config --build-dir=./build --config-file=./config.yaml --validate-resources=files,oci
```

Validate configuration syntax, local files, and OCI artifacts by specifying the option multiple times:

```bash
imagecustomizer validate-config --build-dir=./build --config-file=./config.yaml --validate-resources=files \
  --validate-resources=oci
```

Validate everything:

```bash
imagecustomizer validate-config --build-dir=./build --config-file=./config.yaml --validate-resources=all
```
