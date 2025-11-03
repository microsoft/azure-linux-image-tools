---
parent: Configuration
ancestor: Image Customizer
---

# outputSelinuxPolicy type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `output-selinux-policy` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Specifies the directory path where Image Customizer will output the SELinux policy
contents extracted from the customized image.

The SELinux policy directory (`/etc/selinux/targeted`) is extracted from the image
after customization is complete. This allows downstream components to consume the
SELinux policy files without needing to boot or unpack the entire image.

Example:

```yaml
output:
  selinuxPolicyPath: ./selinux-policy
previewFeatures:
- output-selinux-policy
```

## selinuxPolicyPath [string]

Optional.

Specifies the directory path where Image Customizer will output the SELinux policy
files from `/etc/selinux/targeted`.

The path can be either:
- An absolute path (e.g., `/tmp/selinux-policy`)
- A relative path to the configuration file directory (e.g., `./selinux-policy`)

If this field is not specified or is an empty string, SELinux policy extraction is
disabled.

The output directory will contain a `targeted/` subdirectory with the complete
SELinux policy structure, including:
- `policy/` – Compiled policy files
- `contexts/` – SELinux context configurations
- `seusers` – SELinux user mappings
- Additional policy-related files and directories

If the `/etc/selinux/targeted` directory does not exist in the image, the
customization will fail with an error.

Added in v1.1.
