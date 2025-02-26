---
parent: Configuration
---

# kernelCommandLine type

Options for configuring the kernel.

Added in v0.3.

## extraCommandLine [string[]]

Additional Linux kernel command line options to add to the image.

If bootloader [resetType](./bootloader.md#resettype-string) is set to `"hard-reset"`,
then the `extraCommandLine` value will be appended to the new `grub.cfg` file.

If bootloader [resetType](./bootloader.md#resettype-string) is not set, then the
`extraCommandLine` value will be appended to the existing `grub.cfg` file.

Example:

```yaml
os:
  kernelCommandLine:
    extraCommandLine:
    # Print the system logs to the serial port instead of to the screen, so that they
    # can be programmatically collected.
    - console=tty0
    - console=ttyS0
```

Added in v0.3.
