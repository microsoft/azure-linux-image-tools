# os

Contains the configuration options for the OS.


- [os](#os)
  - [hostname \[string\]](#hostname-string)
  - [bootloader \[bootloader\]](#bootloader-bootloader)
    - [bootLoader type](#bootloader-type)
      - [resetType \[string\]](#resettype-string)
  - [kernelCommandLine \[kernelCommandLine\]](#kernelcommandline-kernelcommandline)
    - [kernelCommandLine type](#kernelcommandline-type)
      - [extraCommandLine \[string\[\]\]](#extracommandline-string)
  - [overlays \[overlay\[\]\]](#overlays-overlay)
    - [overlay type](#overlay-type)
      - [`mountPoint` \[string\]](#mountpoint-string)
      - [`lowerDirs` \[string\[\]\]](#lowerdirs-string)
      - [`upperDir` \[string\]](#upperdir-string)
      - [`workDir` \[string\]](#workdir-string)
      - [`isInitrdOverlay` \[bool\]](#isinitrdoverlay-bool)
      - [`mountDependencies` \[string\[\]\]](#mountdependencies-string)
      - [`mountOptions` \[string\]](#mountoptions-string)
  - [selinux \[selinux\]](#selinux-selinux)
  - [services \[services\]](#services-services)
    - [services type](#services-type)
      - [enable \[string\[\]\]](#enable-string)
      - [disable \[string\[\]\]](#disable-string)
  - [users \[user\]](#users-user)
    - [user type](#user-type)
      - [name \[string\]](#name-string)
      - [uid \[int\]](#uid-int)
      - [password \[password\]](#password-password)
      - [passwordExpiresDays \[int\]](#passwordexpiresdays-int)
      - [sshPublicKeyPaths \[string\[\]\]](#sshpublickeypaths-string)
      - [sshPublicKeys \[string\[\]\]](#sshpublickeys-string)
      - [primaryGroup \[string\]](#primarygroup-string)
      - [secondaryGroups \[string\[\]\]](#secondarygroups-string)
      - [startupCommand \[string\]](#startupcommand-string)
  - [selinux type](#selinux-type)
    - [mode \[string\]](#mode-string)
  - [additionalFiles \[additionalFile\[\]\>\]](#additionalfiles-additionalfile)
    - [additionalFile type](#additionalfile-type)
      - [source \[string\]](#source-string)
      - [content \[string\]](#content-string)
      - [destination \[string\]](#destination-string)
      - [permissions \[string\]](#permissions-string)
  - [additionalDirs \[dirConfig\[\]\]](#additionaldirs-dirconfig)
    - [dirConfig type](#dirconfig-type)
      - [source \[string\]](#source-string-1)
      - [destination \[string\]](#destination-string-1)
      - [newDirPermissions \[string\]](#newdirpermissions-string)
      - [mergedDirPermissions \[string\]](#mergeddirpermissions-string)
      - [childFilePermissions \[string\]](#childfilepermissions-string)
  - [modules \[module\[\]\]](#modules-module)
    - [module type](#module-type)
      - [name \[string\]](#name-string-1)
      - [loadMode \[string\]](#loadmode-string)
      - [options \[map\<string, string\>\]](#options-mapstring-string)
  - [packageList type](#packagelist-type)
    - [installLists \[string\[\]\]](#installlists-string)
    - [removeLists \[string\[\]\]](#removelists-string)
    - [updateLists \[string\[\]\]](#updatelists-string)
  - [packages packages](#packages-packages)
    - [packages \[string\[\]\]](#packages-string)
      - [updateExistingPackages \[bool\]](#updateexistingpackages-bool)
      - [install \[string\[\]\]](#install-string)
      - [remove \[string\[\]\]](#remove-string)
      - [update \[string\[\]\]](#update-string)
  - [password type](#password-type)
    - [type \[string\]](#type-string)
    - [value \[string\]](#value-string)
    - [uki \[uki\]](#uki-uki)
  - [uki type](#uki-type)
    - [kernels](#kernels)

## hostname [string]

Specifies the hostname for the OS.

Implemented by writing to the `/etc/hostname` file.

Example:

```yaml
os:
  hostname: example-image
```

## bootloader [[bootloader](#bootloader-type)]

Defines the configuration for the boot-loader.

### bootLoader type

Defines the configuration for the boot-loader.

#### resetType [string]

Specifies that the boot-loader configuration should be reset and how it should be reset.

Supported options:

- `hard-reset`: Fully reset the boot-loader and its configuration.
  This includes removing any customized kernel command-line arguments that were added to
  base image.

Example:

```yaml
os:
  bootloader:
    resetType: hard-reset
```


<div id="os-kernelcommandline"></div>

## kernelCommandLine [[kernelCommandLine](#kernelcommandline-type)]

Specifies extra kernel command line options.

### kernelCommandLine type

Options for configuring the kernel.

#### extraCommandLine [string[]]

Additional Linux kernel command line options to add to the image.

If bootloader [resetType](#resettype-string) is set to `"hard-reset"`, then the
`extraCommandLine` value will be appended to the new `grub.cfg` file.

If bootloader [resetType](#resettype-string) is not set, then the
`extraCommandLine` value will be appended to the existing `grub.cfg` file.

## overlays [[overlay](#overlay-type)[]]

Used to add filesystem overlays.


### overlay type

Specifies the configuration for overlay filesystem.

Overlays Configuration Example:

```yaml
storage:
  disks:
  bootType: efi
  - partitionTableType: gpt
    maxSize: 4G
    partitions:
    - id: esp
      type: esp
      start: 1M
      end: 9M
    - id: boot
      start: 9M
      end: 108M
    - id: rootfs
      label: rootfs
      start: 108M
      end: 2G
    - id: var
      start: 2G

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077
  - deviceId: boot
    type: ext4
    mountPoint:
      path: /boot
  - deviceId: rootfs
    type: ext4
    mountPoint:
      path: /
  - deviceId: var
    type: ext4
    mountPoint:
      path: /var
      options: defaults,x-initrd.mount

os:
  resetBootLoaderType: hard-reset
  overlays:
    - mountPoint: /etc
      lowerDirs:
      - /etc
      upperDir: /var/overlays/etc/upper
      workDir: /var/overlays/etc/work
      isInitrdOverlay: true
      mountDependencies:
      - /var
    - mountPoint: /media
      lowerDirs:
      - /media
      - /home
      upperDir: /overlays/media/upper
      workDir: /overlays/media/work
```

#### `mountPoint` [string]

The directory where the combined view of the `upperDir` and `lowerDir` will be
mounted. This is the location where users will see the merged contents of the
overlay filesystem. It is common for the `mountPoint` to be the same as the
`lowerDir`. But this is not required.

Example: `/etc`

#### `lowerDirs` [string[]]

These directories act as the read-only layers in the overlay filesystem. They
contain the base files and directories which will be overlaid by the `upperDir`.
Multiple lower directories can be specified by providing a list of paths, which
will be joined using a colon (`:`) as a separator.

Example:

```yaml
lowerDirs:
- /etc
```

#### `upperDir` [string]

This directory is the writable layer of the overlay filesystem. Any
modifications, such as file additions, deletions, or changes, are made in the
upperDir. These changes are what make the overlay filesystem appear different
from the lowerDir alone.

Example: `/var/overlays/etc/upper`

#### `workDir` [string]

This is a required directory used for preparing files before they are merged
into the upperDir. It needs to be on the same filesystem as the upperDir and
is used for temporary storage by the overlay filesystem to ensure atomic
operations. The workDir is not directly accessible to users.

Example: `/var/overlays/etc/work`

#### `isInitrdOverlay` [bool]

A boolean flag indicating whether this overlay is part of the root filesystem.
If set to `true`, specific adjustments will be made, such as prefixing certain
paths with `/sysroot`, and the overlay will be added to the fstab file with the
`x-initrd.mount` option to ensure it is available during the initrd phase.

This is an optional argument.

Example: `False`

#### `mountDependencies` [string[]]

Specifies a list of directories that must be mounted before this overlay. Each
directory in the list should be mounted and available before the overlay
filesystem is mounted.

This is an optional argument.

Example:

```yaml
mountDependencies:
- /var
```

**Important**: If any directory specified in `mountDependencies` needs to be
available during the initrd phase, you must ensure that this directory's mount
configuration in the `filesystems` section includes the `x-initrd.mount` option.
For example:

```yaml
filesystems:
  - deviceId: var
    type: ext4
    mountPoint:
      path: /var
      options: defaults,x-initrd.mount
```

#### `mountOptions` [string]

A string of additional mount options that can be applied to the overlay mount.
Multiple options should be separated by commas.

This is an optional argument.

Example: `noatime,nodiratime`

## selinux [[selinux](#selinux-type)]

Options for configuring SELinux.

Example:

```yaml
os:
  selinux:
    mode: permissive
```

## services [[services](#services-type)]

Options for configuring systemd services.

```yaml
os:
  services:
    enable:
    - sshd
```

### services type

Options for configuring systemd services.

#### enable [string[]]

A list of services to enable.
That is, services that will be set to automatically run on OS boot.

Example:

```yaml
os:
  services:
    enable:
    - sshd
```

#### disable [string[]]

A list of services to disable.
That is, services that will be set to not automatically run on OS boot.

Example:

```yaml
os:
  services:
    disable:
    - sshd
```

## users [[user](#user-type)]

Used to add and/or update user accounts.

Example:

```yaml
os:
  users:
  - name: test
```

### user type

Options for configuring a user account.

<div id="user-name"></div>

#### name [string]

Required.

The name of the user.

Example:

```yaml
os:
  users:
  - name: test
```

#### uid [int]

The ID to use for the user.
This value is not used if the user already exists.

Valid range: 0-60000

Example:

```yaml
os:
  users:
  - name: test
    uid: 1000
```

#### password [[password](#password-type)]

Specifies the user's password.

WARNING: Passwords should not be used in images used in production.

#### passwordExpiresDays [int]

The number of days until the password expires and the user can no longer login.

Valid range: 0-99999. Set to -1 to remove expiry.

Example:

```yaml
os:
  users:
  - name: test
    passwordPath: test-password.txt
    passwordHashed: true
    passwordExpiresDays: 120
```

#### sshPublicKeyPaths [string[]]

A list of file paths to SSH public key files.
These public keys will be copied into the user's `~/.ssh/authorized_keys` file.

Note: It is preferable to use Microsoft Entra ID for SSH authentication, instead of
individual public keys.

Example:

```yaml
os:
  users:
  - name: test
    sshPublicKeyPaths:
    - id_ed25519.pub
```

#### sshPublicKeys [string[]]

A list of SSH public keys.
These public keys will be copied into the user's `~/.ssh/authorized_keys` file.

Note: It is preferable to use Microsoft Entra ID for SSH authentication, instead of
individual public keys.

Example:

```yaml
os:
  users:
  - name: test
    sshPublicKeys:
    - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFyWtgGE06d/uBFQm70tYKvJKwJfRDoh06bWQQwC6Qkm test@test-machine
```

#### primaryGroup [string]

The primary group of the user.

Example:

```yaml
os:
  users:
  - name: test
    primaryGroup: testgroup
```

#### secondaryGroups [string[]]

Additional groups to assign to the user.

Example:

```yaml
os:
  users:
  - name: test
    secondaryGroups:
    - sudo
```

#### startupCommand [string]

The command run when the user logs in.

Example:

```yaml
os:
  users:
  - name: test
    startupCommand: /sbin/nologin
```

## selinux type

### mode [string]

Specifies the mode to set SELinux to.

If this field is not specified, then the existing SELinux mode in the base image is
maintained.
Otherwise, the image is modified to match the requested SELinux mode.

The Azure Linux Image Customizer tool can enable SELinux on a base image with SELinux
disabled and it can disable SELinux on a base image that has SELinux enabled.
However, using a base image that already has the required SELinux mode will speed-up the
customization process.

If SELinux is enabled, then all the file-systems that support SELinux will have their
file labels updated/reset (using the `setfiles` command).

Supported options:

- `disabled`: Disables SELinux.

- `permissive`: Enables SELinux but only logs access rule violations.

- `enforcing`: Enables SELinux and enforces all the access rules.

- `force-enforcing`: Enables SELinux and sets it to enforcing in the kernel
  command-line.
  This means that SELinux can't be set to `permissive` using the `/etc/selinux/config`
  file.

Note: For images with SELinux enabled, the `selinux-policy` package must be installed.
This package contains the default SELinux rules and is required for SELinux-enabled
images to be functional.
The Azure Linux Image Customizer tool will report an error if the package is missing from
the image.

Note: If you wish to apply additional SELinux policies on top of the base SELinux
policy, then it is recommended to apply these new policies using a
([postCustomization](#postcustomization-script)) script.
After applying the policies, you do not need to call `setfiles` manually since it will
called automatically after the `postCustomization` scripts are run.

Example:

```yaml
os:
  selinux:
    mode: enforcing

  packages:
    install:
    # Required packages for SELinux.
    - selinux-policy
    - selinux-policy-modules

    # Optional packages that contain useful SELinux utilities.
    - setools-console
    - policycoreutils-python-utils
```
## additionalFiles [[additionalFile](#additionalfile-type)[]>]

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

### additionalFile type

Specifies options for placing a file in the OS.

Type is used by: [additionalFiles](#additionalfiles-additionalfile)

#### source [string]

The path of the source file to copy to the destination path.

Example:

```yaml
os:
  additionalFiles:
    files/a.txt:
    - path: /a.txt
```

#### content [string]

The contents of the file to write to the destination path.

Example:

```yaml
os:
  additionalFiles:
  - content: |
      abc
    destination: /a.txt
```

#### destination [string]

The absolute path of the destination file.

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
```

#### permissions [string]

The permissions to set on the destination file.

Supported formats:

- String containing an octal string. e.g. `"664"`

Example:

```yaml
os:
  additionalFiles:
  - source: files/a.txt
    destination: /a.txt
    permissions: "664"
```

## additionalDirs [[dirConfig](#dirconfig-type)[]]

Copy directories into the OS image.

This property is a list of [dirConfig](#dirconfig-type) objects.

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

### dirConfig type

Specifies options for placing a directory in the OS.

Type is used by: [additionalDirs](#additionaldirs-dirconfig)

<div id="dirconfig-source"></div>

#### source [string]

The absolute path to the source directory that will be copied.

<div id="dirconfig-destination"></div>

#### destination [string]

The absolute path in the target OS that the source directory will be copied to.

Example:

```yaml
os:
  additionalDirs:
    - source: "home/files/targetDir"
      destination: "usr/project/targetDir"
```

#### newDirPermissions [string]

The permissions to set on all of the new directories being created on the target OS
(including the top-level directory). Default value: `755`.

#### mergedDirPermissions [string]

The permissions to set on the directories being copied that already do exist on the
target OS (including the top-level directory). **Note:** If this value is not specified
in the config, the permissions for this field will be the same as that of the
pre-existing directory.

#### childFilePermissions [string]

The permissions to set on the children file of the directory. Default value: `755`.

Supported formats for permission values:

- String containing an octal value. e.g. `664`

Example:

```yaml
os:
  additionalDirs:
    - source: "home/files/targetDir"
      destination: "usr/project/targetDir"
      newDirPermissions: "644"
      mergedDirPermissions: "777"
      childFilePermissions: "644"
```

## modules [[module](#module-type)[]]

Used to configure kernel modules.

Example:

```yaml
os:
  modules:
    - name: vfio
```

### module type

Options for configuring a kernel module.

<div id="module-name"></div>

#### name [string]

Name of the module.

```yaml
os:
  modules:
  - name: br_netfilter
```

#### loadMode [string]

The loadMode setting for kernel modules dictates how and when these modules
are loaded or disabled in the system.

Supported loadmodes:

- `always`: Set kernel modules to be loaded automatically at boot time.
  - If the module is blacklisted in the base image, remove the blacklist entry.
  - Add the module to `/etc/modules-load.d/modules-load.conf`.
  - Write the options, if provided.

- `auto`: Used for modules that are automatically loaded by the kernel as needed,
    without explicit configuration to load them at boot.
  - If the module is disabled in the base image, remove the blacklist entry to
    allow it to be loaded automatically.
  - Write the provided options to `/etc/modprobe.d/module-options.conf`, but do not
    add the module to `/etc/modules-load.d/modules-load.conf`, as it should be loaded automatically by
    the kernel when necessary.

- `disable`: Configures kernel modules to be explicitly disabled, preventing them from
  loading automatically.
  - If the module is not already disabled in the base image, a blacklist entry will
    be added to `/etc/modprobe.d/blacklist.conf` to ensure the module is disabled.

- `inherit`: Configures kernel modules to inherit the loading behavior set in the base
  image. Only applying new options where they are explicitly provided and applicable.
  - If the module is not disabled, and options are provided, these options will be
    written to `/etc/modprobe.d/module-options.conf`.

-  empty string or not set, it will default to `inherit`.


#### options [map\<string, string>]

Kernel options for modules can specify how these modules interact with the system,
and adjust performance or security settings specific to each module.

```yaml
os:
  modules:
  - name: vfio
    loadMode: always
    options:
      enable_unsafe_noiommu_mode: Y
      disable_vga: Y
```

## packageList type

Used to split off lists of packages into a separate file.
This is useful for sharing list of packages between different configuration files.

This type is used by:

- [installLists](#installlists-string)
- [removeLists](#removelists-string)
- [updateLists](#updatelists-string)

### installLists [string[]]

Same as [install](#install-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](#packagelist-type).

Example:

```yaml
os:
  packages:
    installLists:
    - lists/ssh.yaml
```

### removeLists [string[]]

Same as [remove](#remove-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](#packagelist-type).

Example:

```yaml
os:
  packages:
    removeLists:
    - lists/ssh.yaml
```

### updateLists [string[]]

Same as [update](#update-string) but the packages are specified in a
separate YAML (or JSON) file.

The other YAML file schema is specified by [packageList](#packagelist-type).

Example:

```yaml
os:
  packages:
    updateLists:
    - lists/ssh.yaml
```


## packages [packages](#packages-type)

Remove, update, and install packages on the system.

<div id="os-additionalfiles"></div>

### packages [string[]]

Specifies a list of packages.

Example:

```yaml
packages:
- openssh-server
```

#### updateExistingPackages [bool]

Updates the packages that exist in the base image.

Implemented by calling: `tdnf update`

Example:

```yaml
os:
  packages:
    updateExistingPackages: true
```

#### install [string[]]

Installs packages onto the image.

Implemented by calling: `tdnf install`.

Example:

```yaml
os:
  packages:
    install:
    - openssh-server
```

#### remove [string[]]

Removes packages from the image.

Implemented by calling: `tdnf remove`

Example:

```yaml
os:
  packages:
    remove:
    - openssh-server
```

#### update [string[]]

Updates packages on the system.

Implemented by calling: `tdnf update`

Example:

```yaml
os:
  packages:
    update:
    - openssh-server
```

## password type

Specifies a password for a user.

WARNING: Passwords should not be used in images used in production.

This feature is intended for debugging purposes only.
As such, this feature has been disabled in official builds of the Image Customizer tool.

Instead of using passwords, you should use an authentication system that relies on
cryptographic keys.
For example, SSH with Microsoft Entra ID authentication.

Example:

```yaml
os:
  users:
  - name: test
    password:
      type: locked
```

<div id="password-type-type"></div>

### type [string]

The manner in which the password is provided.

Supported options:

- `locked`: Password login is disabled for the user. This is the default behavior.

Options for debugging purposes only (disabled by default):

- `plain-text`: The value is a plain-text password.

- `hashed`: The value is a password that has been pre-hashed.
  (For example, by using `openssl passwd`.)

- `plain-text-file`: The value is a path to a file containing a plain-text password.

- `hashed-file`: The value is a path to a file containing a pre-hashed password.

<div id="password-type-value"></div>

### value [string]

The password's value.
The meaning of this value depends on the type property.

### uki [[uki](#uki-type)]

Used to create UKI PE images and enable UKI as boot entries.

## uki type

Enables the creation of Unified Kernel Images (UKIs) and configures systemd-boot
to add UKIs as boot entries. UKI combines the Linux kernel, initramfs, kernel
command-line arguments, etc. into a single EFI executable, simplifying system
boot processes and improving security.

If this type is specified, then [os.bootloader.resetType](#resettype-string)
must also be specified.

If this value is specified, then a "uki" entry must be added to
[previewFeatures](#previewfeatures-type)

Example:

```yaml
os:
  bootLoader:
    resetType: hard-reset
  uki:
    kernels: auto
previewFeatures:
- uki
```

### kernels

Specifies which kernels to produce UKIs for.

The value can either contain:

- The string `"auto"`
- A list of kernel version strings.

When `"auto"` is specified, the tool automatically searches for all the
installed kernels and produces UKIs for all the found kernels.

If a list of kernel versions is provided, then the tool will only produce UKIs
for the kernels specified.

The kernel versions must match the regex: `^\d+\.\d+\.\d+(\.\d+)?(-[\w\-\.]+)?$`.
Examples of valid kernel formats: `6.6.51.1-5.azl3`, `5.10.120-4.custom`, `4.18.0-80.el8`.

Example:

```yaml
os:
  uki:
    kernels: auto
```

Example:

```yaml
os:
  uki:
    kernels:
      - 6.6.51.1-5.azl3
      - 5.10.120-4.custom
```
