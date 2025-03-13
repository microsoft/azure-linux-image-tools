---
parent: Configuration
---

# outputVerityHashes type

Specifies the configuration for the output directory containing the generated Verity hash files.

Example:

```yaml
verityHashes:
  path: /output/verityhashes
```

## path [string]

Required.

Specifies the directory path where Prism will output the generated Verity hash files.

### Expected Files in the Directory

After the image customization process, this directory will contain the following unsigned Verity hash files:

- root.hash – The Verity hash for the root filesystem.
- usr.hash – The Verity hash for the user filesystem.

### Generated Verity Hash Files

- These files are not signed—Prism only generates them.
- Signing must be performed externally using a signing service such as ESRP.
- Supported format: .hash (unsigned Verity hash files).
