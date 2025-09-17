---
parent: API
grand_parent: Image Creator
title : Configuration
nav_order: 2
has_toc: false
---

# Image Creator configuration

The Image Creator is configured using a YAML (or JSON) file.

## Top-level

The top level type for the YAML file is the [config](./config.md) type.

## Schema Overview

- [config type](./config.md)
  - [storage](../../../imagecustomizer/api/configuration/config.md#storage-storage)
    - [bootType](../../../imagecustomizer/api/configuration/storage.md#boottype-string)
    - [disks](../../../imagecustomizer/api/configuration/storage.md#disks-disk) ([disk type](../../../imagecustomizer/api/configuration/disk.md))
      - [partitionTableType](../../../imagecustomizer/api/configuration/disk.md#partitiontabletype-string)
      - [maxSize](../../../imagecustomizer/api/configuration/disk.md#maxsize-uint64)
      - [partitions](../../../imagecustomizer/api/configuration/disk.md#partitions-partition) ([partition type](../../../imagecustomizer/api/configuration/partition.md))
        - [id](../../../imagecustomizer/api/configuration/partition.md#id-string)
        - [label](../../../imagecustomizer/api/configuration/partition.md#label-string)
        - [start](../../../imagecustomizer/api/configuration/partition.md#start-uint64)
        - [end](../../../imagecustomizer/api/configuration/partition.md#end-uint64)
        - [size](../../../imagecustomizer/api/configuration/partition.md#size-uint64)
        - [type](../../../imagecustomizer/api/configuration/partition.md#type-string)
    - [filesystems](../../../imagecustomizer/api/configuration/storage.md#filesystems-filesystem) ([filesystem type](../../../imagecustomizer/api/configuration/filesystem.md))
      - [deviceId](../../../imagecustomizer/api/configuration/filesystem.md#deviceid-string)
      - [type](../../../imagecustomizer/api/configuration/filesystem.md#type-string)
      - [mountPoint](../../../imagecustomizer/api/configuration/filesystem.md#mountpoint-mountpoint) ([mountPoint type](../../../imagecustomizer/api/configuration/mountpoint.md))
        - [idType](../../../imagecustomizer/api/configuration/mountpoint.md#idtype-string)
        - [options](../../../imagecustomizer/api/configuration/mountpoint.md#options-string)
        - [path](../../../imagecustomizer/api/configuration/mountpoint.md#path-string)
    - [resetPartitionsUuidsType](../../../imagecustomizer/api/configuration/storage.md#resetpartitionsuuidstype-string)
    - [kernelCommandLine](../../../imagecustomizer/api/configuration/iso.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](../../../imagecustomizer/api/configuration/kernelcommandline.md))
      - [extraCommandLine](../../../imagecustomizer/api/configuration/kernelcommandline.md#extracommandline-string)
    - [initramfsType](../../../imagecustomizer/api/configuration/iso.md#initramfstype-string)
  - [os](../../../imagecustomizer/api/configuration/config.md#os-os) ([os type](../../../imagecustomizer/api/configuration/os.md))
    - [bootloader](../../../imagecustomizer/api/configuration/os.md#bootloader-bootloader) ([bootloader type](../../../imagecustomizer/api/configuration/bootloader.md))
      - [resetType](../../../imagecustomizer/api/configuration/bootloader.md#resettype-string)
    - [hostname](../../../imagecustomizer/api/configuration/os.md#hostname-string)
    - [kernelCommandLine](../../../imagecustomizer/api/configuration/os.md#kernelcommandline-kernelcommandline) ([kernelCommandLine type](../../../imagecustomizer/api/configuration/kernelcommandline.md))
      - [extraCommandLine](../../../imagecustomizer/api/configuration/kernelcommandline.md#extracommandline-string)
    - [packages](../../../imagecustomizer/api/configuration/os.md#packages-packages) ([packages type](../../../imagecustomizer/api/configuration/packages.md))
      - [installLists](../../../imagecustomizer/api/configuration/packages.md#installlists-string)
      - [install](../../../imagecustomizer/api/configuration/packages.md#install-string)
      - [snapshotTime](../../../imagecustomizer/api/configuration/packages.md#snapshottime-string)
    - [additionalFiles](../../../imagecustomizer/api/configuration/os.md#additionalfiles-additionalfile) ([additionalFile type](../../../imagecustomizer/api/configuration/additionalfile.md))
      - [source](../../../imagecustomizer/api/configuration/additionalfile.md#source-string)
      - [content](../../../imagecustomizer/api/configuration/additionalfile.md#content-string)
      - [destination](../../../imagecustomizer/api/configuration/additionalfile.md#destination-string)
      - [permissions](../../../imagecustomizer/api/configuration/additionalfile.md#permissions-string)
    - [additionalDirs](../../../imagecustomizer/api/configuration/os.md#additionaldirs-dirconfig) ([dirConfig type](../../../imagecustomizer/api/configuration/dirconfig.md))
      - [source](../../../imagecustomizer/api/configuration/dirconfig.md#source-string)
      - [destination](../../../imagecustomizer/api/configuration/dirconfig.md#destination-string)
      - [newDirPermissions](../../../imagecustomizer/api/configuration/dirconfig.md#newdirpermissions-string)
      - [mergedDirPermissions](../../../imagecustomizer/api/configuration/dirconfig.md#mergeddirpermissions-string)
      - [childFilePermissions](../../../imagecustomizer/api/configuration/dirconfig.md#childfilepermissions-string)
    - [imageHistory](../../../imagecustomizer/api/configuration/os.md#imagehistory-string)
  - [scripts](../../../imagecustomizer/api/configuration/config.md#scripts-scripts) ([scripts type](../../../imagecustomizer/api/configuration/scripts.md))
    - [postCustomization](../../../imagecustomizer/api/configuration/scripts.md#postcustomization-script) ([script type](../../../imagecustomizer/api/configuration/script.md))
      - [path](../../../imagecustomizer/api/configuration/script.md#path-string)
      - [content](../../../imagecustomizer/api/configuration/script.md#content-string)
      - [interpreter](../../../imagecustomizer/api/configuration/script.md#interpreter-string)
      - [arguments](../../../imagecustomizer/api/configuration/script.md#arguments-string)
      - [environmentVariables](../../../imagecustomizer/api/configuration/script.md#environmentvariables-mapstring-string)
      - [name](../../../imagecustomizer/api/configuration/script.md#name-string)
    - [finalizeCustomization](../../../imagecustomizer/api/configuration/scripts.md#finalizecustomization-script) ([script type](../../../imagecustomizer/api/configuration/script.md))
      - [path](../../../imagecustomizer/api/configuration/script.md#path-string)
      - [content](../../../imagecustomizer/api/configuration/script.md#content-string)
      - [interpreter](../../../imagecustomizer/api/configuration/script.md#interpreter-string)
      - [arguments](../../../imagecustomizer/api/configuration/script.md#arguments-string)
      - [environmentVariables](../../../imagecustomizer/api/configuration/script.md#environmentvariables-mapstring-string)
      - [name](../../../imagecustomizer/api/configuration/script.md#name-string)
  - [previewFeatures type](../../../imagecustomizer/api/configuration/config.md#previewfeatures-string)
  - [output](../../../imagecustomizer/api/configuration/config.md#output-output) ([output type](../../../imagecustomizer/api/configuration/output.md))
    - [image](../../../imagecustomizer/api/configuration/output.md#image-outputimage) ([outputImage type](../../../imagecustomizer/api/configuration/outputImage.md))
      - [path](../../../imagecustomizer/api/configuration/outputImage.md#path-string)
      - [format](../../../imagecustomizer/api/configuration/outputImage.md#format-string)
