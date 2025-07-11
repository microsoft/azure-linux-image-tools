---
parent: Configuration
---

# os type

Contains the configuration options for the OS.

Added in v0.3.

## hostname [string]

Specifies the hostname for the OS.

Implemented by writing to the `/etc/hostname` file.

Example:

```yaml
os:
  hostname: example-image
```

Added in v0.3.

## kernelCommandLine [[kernelCommandLine](./kernelcommandline.md)]

Specifies extra kernel command line options.

Added in v0.3.

## packages [[packages](./packages.md)]

Remove, update, and install packages on the system.

Added in v0.3.

## additionalFiles [[additionalFile](./additionalfile.md)[]]

Copy files into the OS image.

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt

  - content: |
      abc
    destination: /b.txt
    permissions: "664"
```

## additionalDirs [[dirConfig](./dirconfig.md)[]]

Copy directories into the OS image.

This property is a list of [dirConfig](./dirconfig.md) objects.

Example:

```yaml
os:
  additionalDirs:
    # Copying directory with default permission options.
    - source: "path/to/local/directory/"
      destination: "/path/to/destination/directory/"
    # Copying directory with specific permission options.
    - source: "path/to/local/directory/"
      destination: "/path/to/destination/directory/"
      newDirPermissions: 0644
      mergedDirPermissions: 0777
      childFilePermissions: 0644
```

Added in v0.3.

## groups [[group](./group.md)]

Used to add and/or update user groups.

Example:

```yaml
os:
  groups:
  - name: test
```

Added in v0.16.

## users [[user](./user.md)]

Used to add and/or update user accounts.

Example:

```yaml
os:
  users:
  - name: test
```

Added in v0.3.

## modules [[module](./module.md)[]]

Used to configure kernel modules.

Added in v0.3.

## overlays [[overlay](./overlay.md)[]]

Used to add filesystem overlays.

Added in v0.6.

## bootloader [[bootloader](./bootloader.md)]

Defines the configuration for the boot-loader.

Added in v0.8.

## uki [[uki](./uki.md)]

Used to create UKI PE images and enable UKI as boot entries.

Added in v0.8.

## selinux [[selinux](./selinux.md)]

Options for configuring SELinux.

Example:

```yaml
os:
  selinux:
    mode: permissive
```

Added in v0.3.

## services [[services](./services.md)]

Options for configuring systemd services.

```yaml
os:
  services:
    enable:
    - sshd
```

Added in v0.3.

## imageHistory [string]

Options for configuring image history.

Set value to `none` to disable.

To learn more about image history, refer to the [Image History Concept](../../concepts/imagehistory.md) documentation.

```yaml
os:
  imageHistory: none
```

Added in v0.8.
