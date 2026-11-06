---
parent: Configuration
ancestor: Image Customizer
---

# kernelCommandLineReplace type

Removes a set of kernel command-line arguments and replaces them with a new set
of arguments.

Example, replacing the lockdown argument:

```yaml
os: 
  kernelCommandLine:
    replace:
    - remove:
      - lockdown
      add: 
      - lockdown=none 
```

Example, replacing the console arguments:

```yaml
os: 
  kernelCommandLine:
    replace:
    - remove:
      - console
      add: 
      - console=tty0
      - console=ttyS0
```

Example, removing all the console arguments:

```yaml
os: 
  kernelCommandLine: 
    replace:
    - remove:
      - console
```

Added in v1.8.

## remove [string[]]

Optional.

A list of names of kernel command-line arguments to remove.

Any existing arguments with matching names will be removed, regardless of their
value. No error occurs if one or more of the arguments cannot be found.

When modifying the `/etc/default/grub` file, the arguments will be removed
from `GRUB_CMDLINE_LINUX`, `GRUB_CMDLINE_LINUX_DEFAULT`, and
`GRUB_CMDLINE_LINUX_RECOVERY`.

Added in v1.8.

## add [string[]]

Optional.

The list of kernel command-line arguments to add.

If arguments were removed, then the new arguments will be added at the location
of the last removed argument. Otherwise, the new arguments are added at the end.

When modifying the `/etc/default/grub` file, then the arguments will be added to
`GRUB_CMDLINE_LINUX`.

Added in v1.8.
