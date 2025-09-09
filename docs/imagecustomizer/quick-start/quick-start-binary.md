---
title: Quick Start - Binary
parent: Quick Start
grand_parent: Image Customizer
nav_order: 1
---

# Using the Image Customizer Binary

Note: Using the [Image Customizer container](../quick-start/quick-start.md) is the recommended way to use Image Customizer.

## Prerequisities

- Linux host
- Image Customizer binary downloaded. Check out [Developers Guide](../developer-guide.md) to learn how.

## Instructions

1. Download an Azure Linux VHDX image file. 
   - You can [download a marketplace image from Azure](../how-to/azure-vm/download-marketplace-image.md). 
   - You can also download and build one from the [Azure Linux repo](https://github.com/microsoft/azurelinux).

2. Create a customization config file.

   For example:

    ```yaml
    os:
      packages:
        install:
        - dnf
    ```

   For documentation on the supported configuration options, see:
   [Supported configuration](../api/configuration/configuration.md)

3. Install prerequisites: `qemu-img`, `rpm`, `dd`, `lsblk`, `losetup`, `sfdisk`,
   `udevadm`, `flock`, `blkid`, `openssl`, `sed`, `createrepo`, `mksquashfs`,
   `genisoimage`, `mkfs`, `mkfs.ext4`, `mkfs.vfat`, `mkfs.xfs`, `fsck`,
   `e2fsck`, `xfs_repair`, `resize2fs`, `tune2fs`, `xfs_admin`, `fatlabel`, `zstd`,
   `veritysetup`, `grub2-install` (or `grub-install`), `ukify`, `objcopy`, `lsof`.

   - For Ubuntu 22.04 images, run:

     ```bash
     sudo apt -y install qemu-utils rpm coreutils util-linux mount fdisk udev openssl \
        sed createrepo-c squashfs-tools genisoimage e2fsprogs dosfstools \
        xfsprogs zstd cryptsetup-bin grub2-common binutils lsof
     ```

   - For Azure Linux (2.0 and 3.0, x86_64 and arm64), run:

     ```bash
     sudo tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
       sed createrepo_c squashfs-tools cdrkit e2fsprogs dosfstools \
       xfsprogs zstd veritysetup grub2 binutils lsof systemd-ukify
     ```

     - On x86_64, to install libraries for BIOS booting, additionally run:

       ```bash
       sudo tdnf install -y grub2-pc
       ```

       Note: arm64 machines only support UEFI, so the `grub2-pc` package is only needed
       when building x86_64 images.

4. Add executable permissions using `chmod +x imagecustomizer`.

5. Run the Image Customizer tool.

   For example:

    ```bash
    sudo ./imagecustomizer \
      --build-dir ./build \
      --image-file <base-image.vhdx> \
      --output-image-file ./out/image.vhdx \
      --output-image-format vhdx \
      --config-file <config-file.yaml>
    ```

   Where:

   - `<base-image.vhdx>`: The image file downloaded in Step 1.
   - `<config-file.yaml>`: The configuration file created in Step 2.

   For a description of all the command line options, see:
   [Image Customizer command line](../api/cli/cli.md)

   Note: If you are running in WSL (Windows Subsystem for Linux), then you should place
   the `--build-dir` directory in the native Linux filesystem (e.g. `~/build`) instead
   of one of the mounted Windows filesystems (e.g. `/mnt/c`). Otherwise, the tool will
   run very slowly due to I/O performance issues. However, it is fine for `--image-file`
   and `--output-image-file` to be located in either a Windows or Linux filesystem.

   Also, Image Customizer will not run successfully in WSL1. You must use WSL2.

6. Use the customized image.

   The customized image is placed in the file that you specified with the
   `--output-image-file` parameter. You can now use this image as you see fit.
   (For example, boot it in a Hyper-V VM.)

## Next Steps

- Learn how to [deploy the customized image as an Azure VM](../how-to/azure-vm/azure-vm.md)
- Learn more about the [Image Customizer command line](../api/cli/cli.md)
- Learn more about the [Image Customizer config options](../api/configuration/configuration.md)
