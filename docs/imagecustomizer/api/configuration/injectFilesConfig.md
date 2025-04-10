---
parent: Configuration
---

# injectFilesConfig type

Specifies the configuration for injecting files into specified partitions of
an image.

This file is typically generated automatically by Prism when the [`output.artifacts`](./outputArtifacts.md) feature is used. The generated file is named `inject-files.yaml` and placed under the specified output directory. You can modify this file if needed (e.g., to add IPE policies or customize destinations), and later use it with the [`inject-files` CLI command](../cli.md#inject-files) to perform the injection.

Example:

```yaml
injectFiles:
  - partition:
      mountIdType: part-uuid
      id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
    destination: /EFI/Linux/vmlinuz-6.6.51.1-5.azl3.efi
    source: ./vmlinuz-6.6.51.1-5.azl3.signed.efi
    unsignedSource: ./vmlinuz-6.6.51.1-5.azl3.unsigned.efi
  - partition:
      mountIdType: part-uuid
      id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
    destination: /EFI/BOOT/bootx64.efi
    source: ./bootx64.signed.efi
    unsignedSource: ./bootx64.efi
  - partition:
      mountIdType: part-uuid
      id: b9f59ced-d1a6-44a7-91d9-4d623a39b032
    destination: /EFI/systemd/systemd-bootx64.efi
    source: ./systemd-bootx64.signed.efi
    unsignedSource: ./systemd-bootx64.efi
previewFeatures:
  - inject-files
```

## injectFiles [string[]]

Required. 

Specifies a list of files to inject. Each entry defines:

- `partition` - The target partition.
    - `mountIdType` - Type of partition identifier. Supports `uuid`,
      `part-uuid`, and `part-label` currently.
    - `id` - The identifier value (e.g., GPT Partition UUID).
- `destination` - Absolute path inside the mounted partition where the file
  should be copied.
- `source` - Path to the file needed to be copied, can be a relative path to
  this config file or absolute path.
- `unsignedSource` - Path to the unsigned artifact (for reference or auditing
  when signing logic is enabled). This is ignored during the actual injection.

## previewFeatures [string[]]

Required.

Must include `"inject-files"` to enable this preview feature for now.

Added in v0.14.0
