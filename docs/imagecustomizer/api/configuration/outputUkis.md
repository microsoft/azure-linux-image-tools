---
parent: Configuration
---

# outputUkis type

Specifies the configuration for the output directory containing the generated UKI PE images.

Example:

```yaml
ukis:
  path: /output/ukis
```

## path [string]

Required.

Specifies the directory path where Prism will output the generated UKI PE images.

### Expected Files in the Directory

After the image customization process, this directory will contain the following unsigned UKI PE images:

- vmlinuz-<version>.efi – A generated UKI PE image.

### Generated UKI PE Images

- These files are not signed—Prism only generates them.
- Signing must be performed externally using a signing service such as ESRP.
- Supported format: .efi (unsigned UKI PE images).
