---
parent: Configuration
---

# iso type

Specifies the configuration for the generated ISO media.

Example:

```yaml
iso:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt

  kernelCommandLine:
    extraCommandLine:
    - rd.info
```

See also: [ISO Support](../../concepts/iso.md)

## kernelCommandLine [[kernelCommandLine](./kernelcommandline.md)]

Specifies extra kernel command line options.

## additionalFiles [[additionalFile](./additionalfile.md)[]]

Adds files to the ISO.
