---
parent: Configuration
ancestor: Image Customizer
---

# baseConfig type

This is a preview feature.
Its API and behavior is subject to change.
You must enable this feature by specifying `base-configs` in the
[previewFeatures](./config.md#previewfeatures-string) API.

Defines a single configuration file to inherit from.

BaseConfigs specifies a list of base configuration files to inherit from.
When multiple base configs are specified, fields are resolved in order —
fields in later configurations overriding or extending earlier ones.

**The current(last) config’s value (if specified) overrides all base configs.**

- `.input.image.path`  
- `.output.image.path`  
- `.output.image.format`  
- `.output.artifacts.path`

**Base config items are merged with current config’s items**

- `.output.artifacts.items`

## path [string]

A file path to the base config file. The path can be either relative or absolute.
Relative paths are resolved relative to the parent directory of the current config file.

Example:

```yaml
baseConfigs:
  - path: ./base-config.yaml
  - path: /absolute/path/to/base-config.yaml
```

Added in v1.1.0.
