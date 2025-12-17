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

The top level type for the YAML file is the [config](./config.md) type.

## Operation ordering

1. If partitions were specified in the config, customize the disk partitions.

   Otherwise, if the [resetPartitionsUuidsType](./storage.md#resetpartitionsuuidstype-string)
   value is specified, then the partitions' UUIDs are changed.

2. Override the `/etc/resolv.conf` file with the version from the host OS.

3. Update packages:

   1. Remove packages ([removeLists](./packages.md#removelists-string),
      [remove](./packages.md#remove-string))

   2. Update base image packages
      ([updateExistingPackages](./packages.md#updateexistingpackages-bool)).

   3. Install packages ([installLists](./packages.md#installlists-string),
   [install](./packages.md#install-string))

   4. Update packages ([updateLists](./packages.md#updatelists-string),
   [update](./packages.md#update-string))

4. Update hostname. ([hostname](./os.md#hostname-string))

5. Add/update users. ([users](./os.md#users-user))

6. Copy additional directories. ([additionalDirs](./os.md#additionaldirs-dirconfig))

7. Copy additional files. ([additionalFiles](./os.md#additionalfiles-additionalfile))

8. Enable/disable services. ([services](./os.md#services-services))

9. Configure kernel modules. ([modules](./os.md#modules-module))

10. Write the `/etc/image-customizer-release` file.

11. Write the image history file.

12. If the bootloader [resetType](./bootloader.md#resettype-string) is set
    to `hard-reset`, then reset the boot-loader.

    If the bootloader [resetType](./bootloader.md#resettype-string) is not
    set, then append the
    [extraCommandLine](./kernelcommandline.md#extracommandline-string)
    value to the existing `grub.cfg` file.

13. Update the SELinux mode. [mode](./selinux.md#mode-string)

14. If ([overlays](./os.md#overlays-overlay)) are specified, then add the
    overlay driver and update the fstab file with the overlay mount information.

15. If a ([verity](./storage.md#verity-verity)) device is specified, then
    add the dm-verity dracut driver and update the grub config.

16. Regenerate the initramfs file (if needed).

17. Run ([postCustomization](./scripts.md#postcustomization-script)) scripts.

18. Restore the `/etc/resolv.conf` file.

19. If SELinux is enabled, call `setfiles`.

20. Run finalize image scripts. ([finalizeCustomization](./scripts.md#finalizecustomization-script))

21. If `--output-image-format` is `cosi`, then shrink the file systems.

22. If a ([verity](./storage.md#verity-verity)) device is specified, then
    create the hash tree and update the grub config.

23. If ([output.artifacts](./output.md#artifacts-outputartifacts)) is
    specified, then copy the artifacts to the specified output directory.

24. If ([output.selinuxPolicyPath](./output.md#selinuxpolicypath-string)) is
    specified, then extract the SELinux policy files from the customized image.

25. If the output format is set to `iso` or `pxe`, copy additional iso media files.
    ([iso](./iso.md) or [pxe](./pxe.md))

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
a [finalizeCustomization](./scripts.md#finalizecustomization-script) script, since those scripts run
after the `/etc/resolv.conf` is deleted.

## Replacing packages

If you wish to replace a package with conflicting package, then you can remove the
existing package using [remove](./packages.md#remove-string) and then
install the new package with [install](./packages.md#install-string).

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

- [config type](./config.md)
  - [input](./config.md#input-input) ([input type](./input.md))
    - [image](./input.md#image-inputimage) ([inputImage type](./inputImage.md))
      - [path](./inputImage.md#path-string)
  - [storage](./config.md#storage-storage)
    - [bootType](./storage.md#boottype-string)
    - [disks](./storage.md#disks-disk) ([disk type](./disk.md))
      - [partitionTableType](./disk.md#partitiontabletype-string)
      - [maxSize](./disk.md#maxsize-uint64)
      - [partitions](./disk.md#partitions-partition) ([partition type](./partition.md))
        - [id](./partition.md#id-string)
        - [label](./partition.md#label-string)
        - [start](./partition.md#start-uint64)
        - [end](./partition.md#end-uint64)
        - [size](./partition.md#size-uint64)
        - [type](./partition.md#type-string)
    - [verity](./storage.md#verity-verity) ([verity type](./verity.md))
      - [id](./verity.md#id-string)
      - [name](./verity.md#name-string)
      - [dataDeviceId](./verity.md#datadeviceid-string)
      - [hashDeviceId](./verity.md#hashdeviceid-string)
      - [corruptionOption](./verity.md#corruptionoption-string)
    - [filesystems](./storage.md#filesystems-filesystem) ([filesystem type](./filesystem.md))
      - [deviceId](./filesystem.md#deviceid-string)
      - [type](./filesystem.md#type-string)
      - [mountPoint](./filesystem.md#mountpoint-mountpoint) ([mountPoint type](./mountpoint.md))
        - [idType](./mountpoint.md#idtype-string)
        - [options](./mountpoint.md#options-string)
        - [path](./mountpoint.md#path-string)
    - [resetPartitionsUuidsType](./storage.md#resetpartitionsuuidstype-string)
    - [reinitializeVerity](./storage.md#reinitializeverity-string)
  - [iso](./config.md#iso-iso) ([iso type](./iso.md))
    - [additionalFiles](./iso.md#additionalfiles-additionalfile)
      - [additionalFile type](./additionalfile.md)
        - [source](./additionalfile.md#source-string)
        - [content](./additionalfile.md#content-string)
        - [destination](./additionalfile.md#destination-string)
        - [permissions](./additionalfile.md#permissions-string)
    - [kdumpBootFiles](./iso.md#kdumpbootfiles-kdumpbootfiles)
    - [kernelCommandLine](./iso.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./kernelcommandline.md))
      - [extraCommandLine](./kernelcommandline.md#extracommandline-string)
    - [initramfsType](./iso.md#initramfstype-string)
  - [pxe](./config.md#pxe-pxe) ([pxe type](./pxe.md))
    - [additionalFiles](./pxe.md#additionalfiles-additionalfile)
      - [additionalFile type](./additionalfile.md)
        - [source](./additionalfile.md#source-string)
        - [content](./additionalfile.md#content-string)
        - [destination](./additionalfile.md#destination-string)
        - [permissions](./additionalfile.md#permissions-string)
    - [kdumpBootFiles](./pxe.md#kdumpbootfiles-kdumpbootfiles)
    - [kernelCommandLine](./pxe.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./kernelcommandline.md))
      - [extraCommandLine](./kernelcommandline.md#extracommandline-string)
    - [initramfsType](./pxe.md#initramfstype-string)
    - [bootstrapBaseUrl](./pxe.md#bootstrapbaseurl-string)
    - [bootstrapFileUrl](./pxe.md#bootstrapfileurl-string)
  - [os](./config.md#os-os) ([os type](./os.md))
    - [bootloader](./os.md#bootloader-bootloader) ([bootloader type](./bootloader.md))
      - [resetType](./bootloader.md#resettype-string)
    - [hostname](./os.md#hostname-string)
    - [kernelCommandLine](./os.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](./kernelcommandline.md))
      - [extraCommandLine](./kernelcommandline.md#extracommandline-string)
    - [packages](./os.md#packages-packages) ([packages type](./packages.md))
      - [updateExistingPackages](./packages.md#updateexistingpackages-bool)
      - [installLists](./packages.md#installlists-string)
      - [install](./packages.md#install-string)
      - [removeLists](./packages.md#removelists-string)
      - [remove](./packages.md#remove-string)
      - [updateLists](./packages.md#updatelists-string)
      - [update](./packages.md#update-string)
      - [snapshotTime](./packages.md#snapshottime-string)
    - [additionalFiles](./os.md#additionalfiles-additionalfile) ([additionalFile type](./additionalfile.md))
      - [source](./additionalfile.md#source-string)
      - [content](./additionalfile.md#content-string)
      - [destination](./additionalfile.md#destination-string)
      - [permissions](./additionalfile.md#permissions-string)
    - [additionalDirs](./os.md#additionaldirs-dirconfig) ([dirConfig type](./dirconfig.md))
      - [source](./dirconfig.md#source-string)
      - [destination](./dirconfig.md#destination-string)
      - [newDirPermissions](./dirconfig.md#newdirpermissions-string)
      - [mergedDirPermissions](./dirconfig.md#mergeddirpermissions-string)
      - [childFilePermissions](./dirconfig.md#childfilepermissions-string)
    - [groups](./os.md#groups-group) ([group type](./group.md))
      - [name](./group.md#name-string)
      - [gid](./group.md#gid-int)
    - [users](./os.md#users-user) ([user type](./user.md))
      - [name](./user.md#name-string)
      - [uid](./user.md#uid-int)
      - [password](./user.md#password-password) ([password type](./password.md))
        - [type](./password.md#type-string)
        - [value](./password.md#value-string)
      - [passwordExpiresDays](./user.md#passwordexpiresdays-int)
      - [sshPublicKeyPaths](./user.md#sshpublickeypaths-string)
      - [primaryGroup](./user.md#primarygroup-string)
      - [secondaryGroups](./user.md#secondarygroups-string)
      - [startupCommand](./user.md#startupcommand-string)
    - [selinux](./os.md#selinux-selinux) ([selinux type](./selinux.md))
      - [mode](./selinux.md#mode-string)
    - [services](./os.md#services-services) ([selinux type](./services.md))
      - [enable](./services.md#enable-string)
      - [disable](./services.md#disable-string)
    - [modules](./os.md#modules-module) ([module type](./module.md))
      - [name](./module.md#name-string)
      - [loadMode](./module.md#loadmode-string)
      - [options](./module.md#options-mapstring-string)
    - [overlays](./os.md#overlays-overlay) ([overlay type](./overlay.md))
    - [uki](./os.md#uki-uki) ([uki type](./uki.md))
      - [mode](./uki.md#mode-string)
    - [imageHistory](./os.md#imagehistory-string)
  - [scripts](./config.md#scripts-scripts) ([scripts type](./scripts.md))
    - [postCustomization](./scripts.md#postcustomization-script) ([script type](./script.md))
      - [path](./script.md#path-string)
      - [content](./script.md#content-string)
      - [interpreter](./script.md#interpreter-string)
      - [arguments](./script.md#arguments-string)
      - [environmentVariables](./script.md#environmentvariables-mapstring-string)
      - [name](./script.md#name-string)
    - [finalizeCustomization](./scripts.md#finalizecustomization-script) ([script type](./script.md))
      - [path](./script.md#path-string)
      - [content](./script.md#content-string)
      - [interpreter](./script.md#interpreter-string)
      - [arguments](./script.md#arguments-string)
      - [environmentVariables](./script.md#environmentvariables-mapstring-string)
      - [name](./script.md#name-string)
  - [previewFeatures type](./config.md#previewfeatures-string)
  - [output](./config.md#output-output) ([output type](./output.md))
    - [image](./output.md#image-outputimage) ([outputImage type](./outputImage.md))
      - [path](./outputImage.md#path-string)
      - [format](./outputImage.md#format-string)
      - [cosi](./outputImage.md#cosi-cosiconfig) ([cosiConfig type](./cosiConfig.md))
        - [compression](./cosiConfig.md#compression-cosicompression) ([cosiCompression type](./cosiCompression.md))
          - [level](./cosiCompression.md#level-int)
    - [artifacts](./output.md#artifacts-outputartifacts) ([outputArtifacts type](./outputArtifacts.md))
      - [path](./outputArtifacts.md#path-string)
      - [items](./outputArtifacts.md#items-string)
    - [selinuxPolicyPath](./output.md#selinuxpolicypath-string)
