---
parent: Configuration
---

# inputVerityHashes type

Specifies the configuration for inputting signed Verity hash files into the image.

Example:

```yaml
verityHashes:
  path: /input/verityhashes
```

## path [string]

Required.

Specifies the directory path that contains all signed Verity hash files.

### Expected Files in the Directory

The specified directory must contain signed Verity hash files. Typical files found in this directory include:

- root.hash.sig – The signed Verity hash for the root filesystem.
- usr.hash.sig – The signed Verity hash for the user filesystem.

### Signed Verity Hash Files

The files referenced in path should be pre-signed Verity hash files.

- Prism does not perform signing. It is recommended to use ESRP for signing Verity hash files.
- Supported format: .sig (signed Verity hash files).
