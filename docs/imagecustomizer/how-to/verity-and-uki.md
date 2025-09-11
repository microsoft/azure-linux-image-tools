---
title: Create Verity and UKI image
parent: How To
nav_order: 9
---

# Create image with both verity and UKI

This guide demonstrates how to create an image that uses both `/usr` verity and UKI (
Unified Kernel Image).

## Words of caution

UKI is still a preview feature in the Image Customizer tool. The UKI API may change in
the future.

## Steps

1. Create an image config file:

   ```yaml
   # config.yaml
   previewFeatures:
   - uki

   storage:
     bootType: efi
     disks:
     - partitionTableType: gpt
       partitions:
       - id: esp
         type: esp
         size: 250M

       - id: boot
         size: 250M

       - id: usr
         size: 1G

       - id: usrhash
         size: 100M

       - id: root
         size: 2G

     verity:
     - id: verityusr
       name: usr
       dataDeviceId: usr
       hashDeviceId: usrhash
       corruptionOption: panic

     filesystems:
     - deviceId: esp
       type: fat32
       mountPoint:
         path: /boot/efi
         options: umask=0077

     - deviceId: boot
       type: ext4
       mountPoint: /boot

     - deviceId: root
       type: ext4
       mountPoint: /

     - deviceId: verityusr
       type: ext4
       mountPoint:
         path: /usr
         options: ro

   os:
     bootloader:
       resetType: hard-reset

     kernelCommandLine:
       extraCommandLine:
       - rd.info

     uki:
       kernels: auto

     packages:
       remove:
       - grub2-efi-binary

       install:
       - veritysetup
       - systemd-boot
       - device-mapper
    ```

2. Run Image Customizer to create the image file.

    For Hyper-V, run:

    ```bash
    sudo ./imagecustomizer \
      --build-dir ./build \
      --image-file <base-image.vhdx> \
      --output-image-file ./out/image.vhdx \
      --output-image-format vhdx \
      --config-file config.yaml
   ```

    For QEMU, run:

    ```bash
    sudo ./imagecustomizer \
      --build-dir ./build \
      --image-file <base-image.vhdx> \
      --output-image-file ./out/image.qcow2 \
      --output-image-format qcow2 \
      --config-file config.yaml
   ```

   Where:

   - `<base-image.vhdx>`: An official Azure Linux vhdx image file.

## Run image in Hyper-V

1. Create VM:

   ```powershell
   New-VM -Name mytestvm `
     -MemoryStartupBytes 2GB `
     -Generation 2 `
     -BootDevice VHD `
     -SwitchName "Default Switch" `
     -VHDPath '<image.vhdx>'
   Set-VMFirmware -VMName mytestvm -EnableSecureBoot Off
   ```

   Where:

   - `<image.vhdx>`: Is the path of the vhdx file that you generated with Image
     Customizer.

2. Start VM:

   ```powershell
   Start-VM -VMName mytestvm
   ```

## Run image in libvirt/QEMU

1. Create and run VM:

   ```bash
   sudo virt-install \
     --name mytestvm \
     --import \
     --hvm \
     --machine q35 \
     --boot firmware=efi,firmware.feature0.enabled=no,firmware.feature0.name=secure-boot \
     --os-variant linux2024 \
     --disk "<image.qcow2>,device=disk"
   ```

   Where:

   - `<image.qcow2>`: Is the path of the qcow2 file that you generated with Image
     Customizer.

     Note: It is sometimes necessary to move the qcow2 file out of your home directory
     to avoid permissions issues.

## Links

- [Verity Image Recommendations](../concepts/verity/verity.md)
- [Verity API](../api/configuration/verity.md)
- [UKI API](../api/configuration/uki.md)
