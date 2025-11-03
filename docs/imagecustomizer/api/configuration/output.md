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

Added in v0.14.

## selinuxPolicyPath [string]

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-selinux-policy` in the
[previewFeatures](./injectFilesConfig.md#previewfeatures-string) API.

Specifies the directory path where Image Customizer will output the SELinux policy
contents extracted from the customized image.

See [outputSelinuxPolicy](./outputSelinuxPolicy.md) for more details.

Example:

```yaml
output:
  selinuxPolicyPath: ./selinux-policy
previewFeatures:
- output-selinux-policy
```

Added in v1.1.
