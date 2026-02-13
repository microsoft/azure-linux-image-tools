---
parent: Configuration
ancestor: Image Customizer
---

# output type

Specifies the configuration for the output image and artifacts.

## image [[outputImage](./outputImage.md)]

Specifies the configuration for the output image.

Example:

```yaml
output:
  image:
    path: ./out/image.vhdx
    format: vhdx
```

Added in v0.13.

## artifacts [[outputArtifacts](./outputArtifacts.md)]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-artifacts` in the
[previewFeatures](./injectFilesConfig.md#previewfeatures-string) API.

Specifies the configuration for the output directory containing the generated artifacts.

Example:

```yaml
output:
  artifacts:
    items: 
    - ukis
    - shim
    - systemd-boot
    - verity-hash
    path: ./output
previewFeatures:
- output-artifacts
```

Customizing Ubuntu images using this API is not currently tested or supported.

Added in v0.14.

## selinuxPolicyPath [string]

Optional.

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-selinux-policy` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Specifies the directory path where Image Customizer will output the SELinux policy
files. The policy type is determined by reading the `SELINUXTYPE` value from
`/etc/selinux/config` in the image (e.g. `targeted`), and the corresponding directory
from `/etc/selinux/<type>` is extracted.

This field can be overridden by the
[--output-selinux-policy-path](../cli/customize.md#--output-selinux-policy-pathdirectory-path)
command line option. If both are provided, the command line value is used.

The path can be either:

- An absolute path (e.g. `/tmp/selinux-policy`)
- A relative path to the configuration file's parent directory (e.g. `./selinux-policy`)

If this field is not specified or is an empty string, SELinux policy extraction is
disabled.

The output directory will contain a subdirectory named after the `SELINUXTYPE` value
(e.g. `targeted/`) with the complete SELinux policy structure, including:

- `policy/` – Compiled policy files
- `contexts/` – SELinux context configurations
- `seusers` – SELinux user mappings
- Additional policy-related files and directories

If `/etc/selinux/config` does not exist in the image, or if the policy directory
for the configured `SELINUXTYPE` does not exist in `/etc/selinux/`, the
customization will fail with an error.

Example:

```yaml
output:
  selinuxPolicyPath: ./selinux-policy
previewFeatures:
- output-selinux-policy
```

Customizing Ubuntu images using this API is not currently tested or supported.

Added in v1.1.
