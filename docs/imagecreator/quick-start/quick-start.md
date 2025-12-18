---
title: Quick Start
parent: Image Creator
nav_order: 1

---

# Using the Image Creator

## Prerequisites

- Linux host
- Image creator binary downloaded. Check out [Developers Guide](../developer-guide.md) to learn how.

## Instructions

1. Create a image config file.

   For example refer to the below config file

   ```yaml
   storage:
    disks:
    - partitionTableType: gpt
      
      maxSize: 1G
      partitions:
      - id: boot
        type: esp
        start: 1M
        end: 15M

      - id: rootfs
        start: 15M

    bootType: efi

    filesystems:
    - deviceId: boot
      type: fat32
      mountPoint:
         path: /boot/efi
         options: umask=0077

    - deviceId: rootfs
      type: ext4
      mountPoint:
         path: /

   os:
    bootloader:
      resetType: hard-reset

    packages:
      installLists:
      - lists/packages.yaml
   ```

   For documentation on the supported configuration options, see:
   [Supported configuration](../api/configuration/configuration.md)

2. Install prerequisites: `qemu-img`, `rpm`, `dd`, `lsblk`, `losetup`, `sfdisk`,
   `udevadm`, `flock`, `blkid`, `openssl`, `sed`, `createrepo`, `mksquashfs`,
    `mkfs`, `mkfs.ext4`, `mkfs.vfat`, `mkfs.xfs`, `fsck`,
   `e2fsck`, `xfs_repair`, `resize2fs`, `tune2fs`, `xfs_admin`, `fatlabel`, `zstd`,
   `grub2-install` (or `grub-install`), `objcopy`, `lsof`.

   - For Ubuntu 22.04 images, run:

     ```bash
     sudo apt -y install qemu-utils rpm coreutils util-linux mount fdisk udev openssl \
        sed createrepo-c squashfs-tools  e2fsprogs dosfstools \
        xfsprogs zstd cryptsetup-bin grub2-common binutils lsof
     ```

   - For Azure Linux (2.0 and 3.0, x86_64 and arm64), run:

     ```bash
     sudo tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
       sed createrepo_c squashfs-tools cdrkit e2fsprogs dosfstools \
       xfsprogs zstd grub2 binutils lsof
     ```

     - On x86_64, to install libraries for BIOS booting, additionally run:

       ```bash
       sudo tdnf install -y grub2-pc
       ```

       Note: arm64 machines only support UEFI, so the `grub2-pc` package is only needed
       when building x86_64 images.

3. Add executable permissions using `chmod +x imagecreator`.

4. Download the tools file.

   For creating a 2.0 tools file run

    ```bash
    ../../toolkit/tools/internal/testutils/testrpms/create-tools-file.sh \
    "mcr.microsoft.com/cbl-mariner/base/core:2.0" "tools.tar.gz"
    ```

   For creating a 3.0 tools file run

    ```bash
    ../../toolkit/tools/internal/testutils/testrpms/create-tools-file.sh \
    "mcr.microsoft.com/azurelinux/base/core:3.0" "tools.tar.gz"
    ```

   Note: Currently we only support creating Azure Linux 2.0/3.0 images.

5. Run the Image creator tool.

   For example:

    ```bash
    sudo ./imagecreator \
      --build-dir ./build \
      --tools-file <tools-file.tar.gz> \
      --distro azurelinux \
      --distro-version 3.0 \
      --rpm-source azure-linux-rpms.repo \
      --output-image-file ./out/image.vhdx \
      --output-image-format vhdx \
      --config-file <config-file.yaml>
    ```

   Where:

   - `<config-file.yaml>`: The configuration file created in Step 1.
   - `<tools-file.tar.gz>`: The tools file created in step 4.

   For a description of all the command line options, see:
   [Image creator command line](../api/cli.md)

   Note: If you are running in WSL (Windows Subsystem for Linux), then you should place the
   `--build-dir` directory in the native Linux filesystem (e.g. `~/build`) instead of one of the
   mounted Windows filesystems (e.g. `/mnt/c`). Otherwise, the tool will run very slowly due to I/O
   performance issues. However, it is fine for `--output-image-file` to be located in either a
   Windows or Linux filesystem.

   Also, Image creator will not run successfully in WSL1. You must use WSL2.

6. Use the created image.

   The created image is placed in the file that you specified with the
   `--output-image-file` parameter. You can now use this image as you see fit.
   (For example, boot it in a Hyper-V VM.)

- Learn more about the [Image creator command line](../api/cli.md)
- Learn more about the [Image creator config options](../api/configuration/configuration.md)
