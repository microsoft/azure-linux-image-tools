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

The SELinux policy directory is extracted from the image after customization is
complete. The policy type is determined by reading the `SELINUXTYPE` value from
`/usr/etc/selinux/config` in the image, and the corresponding policy directory 
(e.g., `/usr/etc/selinux/targeted`) is extracted. This allows downstream components
to consume the SELinux policy files without needing to boot or unpack the entire image.

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
files. The policy type is determined by reading the `SELINUXTYPE` value from
`/usr/etc/selinux/config` in the image (e.g., `targeted`), and the corresponding directory
from `/usr/etc/selinux/<type>` is extracted.

This field can be overridden by the
[--output-selinux-policy-path](../cli/cli.md#--output-selinux-policy-pathdirectory-path)
command line option. If both are provided, the command line value is used.

The path can be either:
- An absolute path (e.g., `/tmp/selinux-policy`)
- A relative path to the configuration file directory (e.g., `./selinux-policy`)

If this field is not specified or is an empty string, SELinux policy extraction is
disabled.

The output directory will contain a subdirectory named after the `SELINUXTYPE` value
(e.g., `targeted/`) with the complete SELinux policy structure, including:
- `policy/` – Compiled policy files
- `contexts/` – SELinux context configurations
- `seusers` – SELinux user mappings
- Additional policy-related files and directories

If `/usr/etc/selinux/config` does not exist in the image, or if the policy directory
for the configured `SELINUXTYPE` does not exist in `/usr/etc/selinux/`, the
customization will fail with an error.

Added in v1.1.
