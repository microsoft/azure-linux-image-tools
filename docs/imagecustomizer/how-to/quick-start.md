---
parent: How To
nav_order: 4
---

# Quick start

1. Download an Azure Linux core VHDX image file.

2. Create a customization config file.

   For example:

    ```yaml
    os:
      packagesInstall:
      - dnf
    ```

   For documentation on the supported configuration options, see:
   [Supported configuration](./api/configuration.md)

3. Install prerequisites: `qemu-img`, `rpm`, `dd`, `lsblk`, `losetup`, `sfdisk`,
   `udevadm`, `flock`, `blkid`, `openssl`, `sed`, `createrepo`, `mksquashfs`,
   `genisoimage`, `parted`, `mkfs`, `mkfs.ext4`, `mkfs.vfat`, `mkfs.xfs`, `fsck`,
   `e2fsck`, `xfs_repair`, `resize2fs`, `tune2fs`, `xfs_admin`, `fatlabel`, `zstd`,
   `veritysetup`, `grub2-install` (or `grub-install`).

   - For Ubuntu 22.04 images, run:

     ```bash
     sudo apt -y install qemu-utils rpm coreutils util-linux mount fdisk udev openssl \
        sed createrepo-c squashfs-tools genisoimage parted e2fsprogs dosfstools \
        xfsprogs zstd cryptsetup-bin grub2-common
     ```

   - For Azure Linux 2.0 and 3.0, run:

     ```bash
     sudo tdnf install -y qemu-img rpm coreutils util-linux systemd openssl \
        sed createrepo_c squashfs-tools cdrkit parted e2fsprogs dosfstools \
        xfsprogs zstd veritysetup grub2 grub2-pc
     ```

4. Run the Image Customizer tool.

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
   [Image Customizer command line](./api/cli.md)

5. Use the customized image.

   The customized image is placed in the file that you specified with the
   `--output-image-file` parameter. You can now use this image as you see fit.
   (For example, boot it in a Hyper-V VM.)