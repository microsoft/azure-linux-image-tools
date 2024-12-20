
# os type

Contains the configuration options for the OS.

## hostname [string]

Specifies the hostname for the OS.

Implemented by writing to the `/etc/hostname` file.

Example:

```yaml
os:
  hostname: example-image
```

## kernelCommandLine [[kernelCommandLine](./kernelcommandline.md)]

Specifies extra kernel command line options.

## packages [[packages](./packages.md)]

Remove, update, and install packages on the system.

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

## users [[user](./user.md)]

Used to add and/or update user accounts.

Example:

```yaml
os:
  users:
  - name: test
```

## modules [[module](./module.md)[]]

Used to configure kernel modules.

Example:

```yaml
os:
  modules:
    - name: vfio
```

## overlays [[overlay](./overlay.md)[]]

Used to add filesystem overlays.

## bootloader [[bootloader](./bootloader.md)]

Defines the configuration for the boot-loader.

## uki [[uki](./uki.md)]

Used to create UKI PE images and enable UKI as boot entries.

## selinux [[selinux](./selinux.md)]

Options for configuring SELinux.

Example:

```yaml
os:
  selinux:
    mode: permissive
```

## services [[services](./services.md)]

Options for configuring systemd services.

```yaml
os:
  services:
    enable:
    - sshd
```

## imageHistory [string]

Options for configuring image history.

Set value to `none` to disable.

```yaml
os:
  imageHistory: none
```
