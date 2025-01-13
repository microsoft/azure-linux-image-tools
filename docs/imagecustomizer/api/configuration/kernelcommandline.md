# kernelCommandLine type

Options for configuring the kernel.

## extraCommandLine [string[]]

Additional Linux kernel command line options to add to the image.

If bootloader [resetType](./bootloader.md#resettype-string) is set to `"hard-reset"`,
then the `extraCommandLine` value will be appended to the new `grub.cfg` file.

If bootloader [resetType](./bootloader.md#resettype-string) is not set, then the
`extraCommandLine` value will be appended to the existing `grub.cfg` file.
