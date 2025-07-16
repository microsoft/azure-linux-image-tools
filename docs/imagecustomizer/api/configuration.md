---
title: Configuration
parent: API
grand_parent: Image Customizer
nav_order: 2
has_toc: false
---

# Image Customizer configuration

The Image Customizer is configured using a YAML (or JSON) file.

## Top-level

The top level type for the YAML file is the [config](./configuration/config.md) type.

## Operation ordering

1. If partitions were specified in the config, customize the disk partitions.

   Otherwise, if the [resetPartitionsUuidsType](./configuration/storage.md#resetpartitionsuuidstype-string)
   value is specified, then the partitions' UUIDs are changed.

2. Override the `/etc/resolv.conf` file with the version from the host OS.

3. Update packages:

   1. Remove packages ([removeLists](./configuration/packages.md#removelists-string),
      [remove](./configuration/packages.md#remove-string))

   2. Update base image packages
      ([updateExistingPackages](./configuration/packages.md#updateexistingpackages-bool)).

   3. Install packages ([installLists](./configuration/packages.md#installlists-string),
   [install](./configuration/packages.md#install-string))

   4. Update packages ([updateLists](./configuration/packages.md#updatelists-string),
   [update](./configuration/packages.md#update-string))

4. Update hostname. ([hostname](./configuration/os.md#hostname-string))

5. Add/update users. ([users](./configuration/os.md#users-user))

6. Copy additional directories. ([additionalDirs](./configuration/os.md#additionaldirs-dirconfig))

7. Copy additional files. ([additionalFiles](./configuration/os.md#additionalfiles-additionalfile))

8. Enable/disable services. ([services](./configuration/os.md#services-services))

9. Configure kernel modules. ([modules](./configuration/os.md#modules-module))

10. Write the `/etc/image-customizer-release` file.

11. Write the image history file.

12. If the bootloader [resetType](./configuration/bootloader.md#resettype-string) is set
    to `hard-reset`, then reset the boot-loader.

    If the bootloader [resetType](./configuration/bootloader.md#resettype-string) is not
    set, then append the
    [extraCommandLine](./configuration/kernelcommandline.md#extracommandline-string)
    value to the existing `grub.cfg` file.

13. Update the SELinux mode. [mode](./configuration/selinux.md#mode-string)

14. If ([overlays](./configuration/os.md#overlays-overlay)) are specified, then add the
    overlay driver and update the fstab file with the overlay mount information.

15. If a ([verity](./configuration/storage.md#verity-verity)) device is specified, then
    add the dm-verity dracut driver and update the grub config.

16. Regenerate the initramfs file (if needed).

17. Run ([postCustomization](./configuration/scripts.md#postcustomization-script)) scripts.

18. Restore the `/etc/resolv.conf` file.

19. If SELinux is enabled, call `setfiles`.

20. Run finalize image scripts. ([finalizeCustomization](./configuration/scripts.md#finalizecustomization-script))

21. If `--output-image-format` is `cosi`, then shrink the file systems.

22. If a ([verity](./configuration/storage.md#verity-verity)) device is specified, then
    create the hash tree and update the grub config.

23. If the output format is set to `iso` or `pxe`, copy additional iso media files.
    ([iso](./configuration/iso.md) or [pxe](./configuration/pxe.md))

## /etc/resolv.conf

The `/etc/resolv.conf` file is overridden during customization so that the package
installation and customization scripts can have access to the network.

Near the end of customization, the `/etc/resolv.conf` file is restored to its original
state.

However, if the `/etc/resolv.conf` did not exist in the base image and
`systemd-resolved` service is enabled, then the `/etc/resolv.conf` file is symlinked to
the `/run/systemd/resolve/stub-resolv.conf` file. (This would happen anyway during
first-boot. But doing this during customization is useful for verity enabled images
where the filesystem is readonly.)

If you want to explicitly set the `/etc/resolv.conf` file contents, you can do so within
a [finalizeCustomization](./configuration/scripts.md#finalizecustomization-script) script, since those scripts run
after the `/etc/resolv.conf` is deleted.

## Replacing packages

If you wish to replace a package with conflicting package, then you can remove the
existing package using [remove](./configuration/packages.md#remove-string) and then
install the new package with [install](./configuration/packages.md#install-string).

Example:

```yaml
os:
  packages:
    remove:
    - kernel

    install:
    - kernel-uvm
```

## Schema Overview

- [config type](./configuration/config.md)
  - [input](./configuration/config.md#input-input) ([input type](./configuration/input.md))
    - [image](./configuration/input.md#image-inputimage) ([inputImage type](./configuration/inputImage.md))
      - [path](./configuration/inputImage.md#path-string)
  - [storage](./configuration/config.md#storage-storage)
    - [bootType](./configuration/storage.md#boottype-string)
    - [disks](./configuration/storage.md#disks-disk) ([disk type](./configuration/disk.md))
      - [partitionTableType](./configuration/disk.md#partitiontabletype-string)
      - [maxSize](./configuration/disk.md#maxsize-uint64)
      - [partitions](./configuration/disk.md#partitions-partition) ([partition type](./configuration/partition.md))
        - [id](./configuration/partition.md#id-string)
        - [label](./configuration/partition.md#label-string)
        - [start](./configuration/partition.md#start-uint64)
        - [end](./configuration/partition.md#end-uint64)
        - [size](./configuration/partition.md#size-uint64)
        - [type](./configuration/partition.md#type-string)
    - [verity](./configuration/storage.md#verity-verity) ([verity type](./configuration/verity.md))
      - [id](./configuration/verity.md#id-string)
      - [name](./configuration/verity.md#name-string)
      - [dataDeviceId](./configuration/verity.md#datadeviceid-string)
      - [hashDeviceId](./configuration/verity.md#hashdeviceid-string)
      - [corruptionOption](./configuration/verity.md#corruptionoption-string)
    - [filesystems](./configuration/storage.md#filesystems-filesystem) ([filesystem type](./configuration/filesystem.md))
      - [deviceId](./configuration/filesystem.md#deviceid-string)
      - [type](./configuration/filesystem.md#type-string)
      - [mountPoint](./configuration/filesystem.md#mountpoint-mountpoint) ([mountPoint type](./configuration/mountpoint.md))
        - [idType](./configuration/mountpoint.md#idtype-string)
        - [options](./configuration/mountpoint.md#options-string)
        - [path](./configuration/mountpoint.md#path-string)
    - [resetPartitionsUuidsType](./configuration/storage.md#resetpartitionsuuidstype-string)
  - [iso](./configuration/config.md#iso-iso) ([iso type](./configuration/iso.md))
    - [additionalFiles](./configuration/iso.md#additionalfiles-additionalfile)
      - [additionalFile type](./configuration/additionalfile.md)
        - [source](./configuration/additionalfile.md#source-string)
        - [content](./configuration/additionalfile.md#content-string)
        - [destination](./configuration/additionalfile.md#destination-string)
        - [permissions](./configuration/additionalfile.md#permissions-string)
    - [kdumpBootFiles](./configuration/iso.md#kdumpbootfiles-kdumpbootfiles)
    - [kernelCommandLine](./configuration/iso.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./configuration/kernelcommandline.md))
      - [extraCommandLine](./configuration/kernelcommandline.md#extracommandline-string)
    - [initramfsType](./configuration/iso.md#initramfstype-string)
  - [pxe](./configuration/config.md#pxe-pxe) ([pxe type](./configuration/pxe.md))
    - [additionalFiles](./configuration/pxe.md#additionalfiles-additionalfile)
      - [additionalFile type](./configuration/additionalfile.md)
        - [source](./configuration/additionalfile.md#source-string)
        - [content](./configuration/additionalfile.md#content-string)
        - [destination](./configuration/additionalfile.md#destination-string)
        - [permissions](./configuration/additionalfile.md#permissions-string)
    - [kdumpBootFiles](./configuration/pxe.md#kdumpbootfiles-kdumpbootfiles)
    - [kernelCommandLine](./configuration/pxe.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./configuration/kernelcommandline.md))
      - [extraCommandLine](./configuration/kernelcommandline.md#extracommandline-string)
    - [initramfsType](./configuration/pxe.md#initramfstype-string)
    - [bootstrapBaseUrl](./configuration/pxe.md#bootstrapbaseurl-string)
    - [bootstrapFileUrl](./configuration/pxe.md#bootstrapfileurl-string)
  - [os](./configuration/config.md#os-os) ([os type](./configuration/os.md))
    - [bootloader](./configuration/os.md#bootloader-bootloader) ([bootloader type](./configuration/bootloader.md))
      - [resetType](./configuration/bootloader.md#resettype-string)
    - [hostname](./configuration/os.md#hostname-string)
    - [kernelCommandLine](./configuration/os.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./configuration/kernelcommandline.md))
      - [extraCommandLine](./configuration/kernelcommandline.md#extracommandline-string)
    - [packages](./configuration/os.md#packages-packages) ([packages type](./configuration/packages.md))
      - [updateExistingPackages](./configuration/packages.md#updateexistingpackages-bool)
      - [installLists](./configuration/packages.md#installlists-string)
      - [install](./configuration/packages.md#install-string)
      - [removeLists](./configuration/packages.md#removelists-string)
      - [remove](./configuration/packages.md#remove-string)
      - [updateLists](./configuration/packages.md#updatelists-string)
      - [update](./configuration/packages.md#update-string)
      - [snapshotTime](./configuration/packages.md#snapshottime-string)
    - [additionalFiles](./configuration/os.md#additionalfiles-additionalfile) ([additionalFile type](./configuration/additionalfile.md))
      - [source](./configuration/additionalfile.md#source-string)
      - [content](./configuration/additionalfile.md#content-string)
      - [destination](./configuration/additionalfile.md#destination-string)
      - [permissions](./configuration/additionalfile.md#permissions-string)
    - [additionalDirs](./configuration/os.md#additionaldirs-dirconfig) ([dirConfig type](./configuration/dirconfig.md))
      - [source](./configuration/dirconfig.md#source-string)
      - [destination](./configuration/dirconfig.md#destination-string)
      - [newDirPermissions](./configuration/dirconfig.md#newdirpermissions-string)
      - [mergedDirPermissions](./configuration/dirconfig.md#mergeddirpermissions-string)
      - [childFilePermissions](./configuration/dirconfig.md#childfilepermissions-string)
    - [groups](./configuration/os.md#groups-group) ([group type](./configuration/group.md))
      - [name](./configuration/group.md#name-string)
      - [gid](./configuration/group.md#gid-int)
    - [users](./configuration/os.md#users-user) ([user type](./configuration/user.md))
      - [name](./configuration/user.md#name-string)
      - [uid](./configuration/user.md#uid-int)
      - [password](./configuration/user.md#password-password) ([password type](./configuration/password.md))
        - [type](./configuration/password.md#type-string)
        - [value](./configuration/password.md#value-string)
      - [passwordExpiresDays](./configuration/user.md#passwordexpiresdays-int)
      - [sshPublicKeyPaths](./configuration/user.md#sshpublickeypaths-string)
      - [primaryGroup](./configuration/user.md#primarygroup-string)
      - [secondaryGroups](./configuration/user.md#secondarygroups-string)
      - [startupCommand](./configuration/user.md#startupcommand-string)
    - [selinux](./configuration/os.md#selinux-selinux) ([selinux type](./configuration/selinux.md))
      - [mode](./configuration/selinux.md#mode-string)
    - [services](./configuration/os.md#services-services) ([selinux type](./configuration/services.md))
      - [enable](./configuration/services.md#enable-string)
      - [disable](./configuration/services.md#disable-string)
    - [modules](./configuration/os.md#modules-module) ([module type](./configuration/module.md))
      - [name](./configuration/module.md#name-string)
      - [loadMode](./configuration/module.md#loadmode-string)
      - [options](./configuration/module.md#options-mapstring-string)
    - [overlays](./configuration/os.md#overlays-overlay) ([overlay type](./configuration/overlay.md))
    - [uki](./configuration/os.md#uki-uki) ([uki type](./configuration/uki.md))
      - [kernels](./configuration/uki.md#kernels)
    - [imageHistory](./configuration/os.md#imagehistory-string)
  - [scripts](./configuration/config.md#scripts-scripts) ([scripts type](./configuration/scripts.md))
    - [postCustomization](./configuration/scripts.md#postcustomization-script) ([script type](./configuration/script.md))
      - [path](./configuration/script.md#path-string)
      - [content](./configuration/script.md#content-string)
      - [interpreter](./configuration/script.md#interpreter-string)
      - [arguments](./configuration/script.md#arguments-string)
      - [environmentVariables](./configuration/script.md#environmentvariables-mapstring-string)
      - [name](./configuration/script.md#name-string)
    - [finalizeCustomization](./configuration/scripts.md#finalizecustomization-script) ([script type](./configuration/script.md))
      - [path](./configuration/script.md#path-string)
      - [content](./configuration/script.md#content-string)
      - [interpreter](./configuration/script.md#interpreter-string)
      - [arguments](./configuration/script.md#arguments-string)
      - [environmentVariables](./configuration/script.md#environmentvariables-mapstring-string)
      - [name](./configuration/script.md#name-string)
  - [previewFeatures type](./configuration/config.md#previewfeatures-string)
  - [output](./configuration/config.md#output-output) ([output type](./configuration/output.md))
    - [image](./configuration/output.md#image-outputimage) ([outputImage type](./configuration/outputImage.md))
      - [path](./configuration/outputImage.md#path-string)
      - [format](./configuration/outputImage.md#format-string)
    - [artifacts](./configuration/output.md#artifacts-outputartifacts) ([outputArtifacts type](./configuration/outputArtifacts.md))
      - [path](./configuration/outputArtifacts.md#path-string)
      - [items](./configuration/outputArtifacts.md#items-string)
