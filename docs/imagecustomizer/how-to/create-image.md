---
title: Create Image
parent: How To
nav_order: 8
has_toc: false
---

# Creating a New Image from Scratch

This guide shows how to use the Image Customizer [create subcommand](../api/cli/create.md) to build a new Linux
image from scratch.

This command is a preview feature.
Its API and behavior is subject to change.
As such, the example configuration files below must specify `create` in the [previewFeatures](../api/configuration/config.md#previewfeatures-string) field.

## Prerequisites

- Linux host
- Docker (or equivalent container engine) installed on your host

## Instructions

1. Create a configuration file.

    This file has the same API as configuration files for the [customize subcommand](../api/cli/customize.md), but must
    contain all the settings needed to create the image from scratch.

    For Azure Linux 3.0, create `$HOME/staging/config-azl3.yaml`:

    ```yaml
    previewFeatures:
    - create

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
        install:
        - azurelinux-release
        - azurelinux-repos
        - azurelinux-rpm-macros
        - bash
        - ca-certificates
        - ca-certificates-base
        - dbus
        - e2fsprogs
        - filesystem
        - grub2
        - grub2-efi-binary
        - iana-etc
        - initramfs
        - iproute
        - iputils
        - irqbalance
        - ncurses-libs
        - openssl
        - rpm
        - rpm-libs
        - shadow-utils
        - shim
        - sudo
        - systemd
        - systemd-networkd
        - systemd-resolved
        - systemd-udev
        - tdnf
        - tdnf-plugin-repogpgcheck
        - util-linux
        - zlib
        - kernel
    ```

    For Fedora 42, create `$HOME/staging/config-fedora.yaml`:

    ```yaml
    previewFeatures:
    - create
    - fedora

    storage:
      disks:
      - partitionTableType: gpt
        maxSize: 2G
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
        install:
        - fedora-release
        - fedora-repos
        - fedora-rpm-macros
        - bash
        - ca-certificates
        - dbus
        - e2fsprogs
        - filesystem
        - grub2-common
        - grub2-efi-x64
        - setup
        - dracut-config-generic
        - iproute
        - iputils
        - irqbalance
        - ncurses-libs
        - openssl
        - rpm
        - rpm-libs
        - shadow-utils
        - shim-x64
        - sudo
        - systemd
        - systemd-networkd
        - systemd-resolved
        - systemd-udev
        - dnf5
        - util-linux
        - zlib-ng-compat
        - kernel
    ```

   For documentation on the supported configuration options, see:
   [Supported configuration](../api/configuration/configuration.md)

2. Create the tools directory.

   The tools directory contains the bootstrap utilities (tdnf and its dependencies)
   needed to install packages during image creation.

   For step-by-step instructions, see:
   [How to create the tools directory](../how-to/create-tools-dir.md)

3. Create a repository configuration file and, if needed, a GPG key file.

   The repository configuration file (`.repo` file) tells the Image Customizer where to download RPM packages from.

   For Azure Linux 3.0, create `$HOME/staging/azure-linux-3.0-rpms.repo` with the following contents:

    ```ini
    [azurelinux-base]
    name=Azure Linux 3.0 Base
    baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/base/$basearch/
    gpgcheck=1
    repo_gpgcheck=1
    enabled=1
    gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
    ```

   For Fedora 42, create `$HOME/staging/fedora-42-rpms.repo` with the following contents:
   
    ```ini
    [fedora]
    name=Fedora 42 - $basearch
    baseurl=https://dl.fedoraproject.org/pub/fedora/linux/releases/42/Everything/$basearch/os/
    metalink=https://mirrors.fedoraproject.org/metalink?repo=fedora-42&arch=$basearch
    enabled=1
    gpgcheck=1
    gpgkey=file:///mnt/staging/fedora-42-rpm-gpg-key

    [fedora-updates]
    name=Fedora $releasever - $basearch - Updates
    baseurl=https://dl.fedoraproject.org/pub/fedora/linux/updates/$releasever/Everything/$basearch/
    metalink=https://mirrors.fedoraproject.org/metalink?repo=updates-released-f$releasever&arch=$basearch
    enabled=1
    gpgcheck=1
    gpgkey=file:///mnt/staging/fedora-42-rpm-gpg-key
    ```

4. Download GPG keys (Fedora 42 only).

   Securely download and verify the Fedora 42 RPM GPG key from the [Fedora Project](https://fedoraproject.org/security).
   Save it to `$HOME/staging/fedora-42-rpm-gpg-key`.

5. Run the Image Customizer container. Here is a sample command to run it:

   To create a new Azure Linux 3.0 image:

    ```bash
    docker run \
      --rm \
      --privileged=true \
      -v "$HOME/staging:/mnt/staging:z" \
      mcr.microsoft.com/azurelinux/imagecustomizer:latest create \
        --distro azurelinux \
        --distro-version 3.0 \
        --tools-dir /mnt/staging/tools-azurelinux-3.0 \
        --rpm-source /mnt/staging/azure-linux-3.0-rpms.repo \
        --config-file /mnt/staging/config-azl3.yaml \
        --build-dir /build \
        --output-image-format vhdx \
        --output-image-file /mnt/staging/out/azure-linux-3.0-image.vhdx
    ```

   To create a new Fedora 42 image:

    ```bash
    docker run \
      --rm \
      --privileged=true \
      -v "$HOME/staging:/mnt/staging:z" \
      mcr.microsoft.com/azurelinux/imagecustomizer:latest create \
        --distro fedora \
        --distro-version 42 \
        --tools-dir /mnt/staging/tools-fedora-42 \
        --rpm-source /mnt/staging/fedora-42-rpms.repo \
        --config-file /mnt/staging/config-fedora.yaml \
        --build-dir /build \
        --output-image-format vhdx \
        --output-image-file /mnt/staging/out/fedora-42-image.vhdx
    ```

    Docker options:

    - `run`: Runs the container.

    - `--rm`: Cleans up the container once the program has completed.

    - `--privileged=true`: Gives the container root permissions, which is needed to mount
      loopback devices (i.e. disk files) and partitions.

      This option maps in the host's version of `/dev` into the container, instead of the
      container getting its own `/dev`.

    - `-v $HOME/staging:/mnt/staging:z`: Mounts a host directory (`$HOME/staging`) into the
      container. This can be used to easily pass files in and out of the container.

    - `mcr.microsoft.com/azurelinux/imagecustomizer:latest`: The container to run.

    - `create`: Specifies the subcommand to run within the container.

    For v1.4 and below, you must also add the docker option:

    - `-v /dev:/dev`: When mounting loopback devices, the container needs the partition
      device nodes to be populated under `/dev`. But the container only has a copy of
      `/dev` at the time of the container's creation.

    Image Customizer options for the ([create subcommand](../api/cli/create.md)):

    - `--distro fedora`: Create a Fedora image.

    - `--distro-version 42`: Specify the Fedora version to create the image for.

    - `--tools-dir /mnt/staging/tools-fedora-42`: Use the host's
      `$HOME/staging/tools-fedora-42` directory as the bootstrapping tools directory.

    - `--rpm-source /mnt/staging/fedora-42-rpms.repo`: Use the host's
      `$HOME/staging/fedora-42-rpms.repo` file for installing packages.

    - `--config-file /mnt/staging/image-config.yaml`: Use the host's
      `$HOME/staging/image-config.yaml` file as the config.

    - `--build-dir /build`: Use `/build` inside the container as the build directory.
      (This directory is ephemeral and will be deleted when the container exits.)

    - `--output-image-format vhdx`: Output the created image as a VHDX file.

    - `--output-image-file /mnt/staging/out/image.vhdx`: Output the created image to
      the host's `$HOME/staging/out/image.vhdx` file path.

6. Use the created image.

   The created image is placed in the file that you specified with the
   `--output-image-file` parameter. You can now use this image as you see fit.
   (For example, boot it in a Hyper-V VM.)

## Next Steps

- Learn how to [deploy the created image as an Azure VM](../how-to/azure-vm/azure-vm.md)
- Learn more about the [create subcommand](../api/cli/create.md)
- Learn more about the [configuration options](../api/configuration/configuration.md)
