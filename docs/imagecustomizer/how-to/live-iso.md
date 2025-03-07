---
title: Create LiveOS ISO
parent: How To
nav_order: 9
---

# Create a LiveOS ISO and run it in a VM

This guide demonstrates how to use Image Customizer to build an Azure Linux LiveOS ISO
and then run it in a local VM.

A LiveOS image is an OS image that can be booted from media (e.g. CDROM, USB) without
needing to install the OS.

## Build LiveOS ISO

1. Create image config file.

   The following config file creates a service that runs after boot and prints a message
   to the system console:

   ```yaml
   # config.yaml
   os:
     # Install ISO required packages
     packages:
       install:
       - squashfs-tools
       - tar
       - device-mapper
       - curl

     additionalFiles:
     - destination: /usr/local/lib/systemd/system/myservice.service
       content: |
         [Unit]
         Description=My Service

         [Service]
         Type=exec
         ExecStart=/usr/local/bin/myservice.sh
         # Print to the system console.
         StandardOutput=journal+console

         [Install]
         WantedBy=multi-user.target

     - destination: /usr/local/bin/myservice.sh
       permissions: 755
       content: |
         #!/usr/bin/env bash
         set -eu

         # Sleep for a bit so that the message is easy to see.
         sleep 10
         echo "Hello, World!"

     services:
       enable:
       - myservice
   ```

2. Run Image Customizer to create the LiveOS ISO file:

    ```bash
    sudo ./imagecustomizer \
      --build-dir ./build \
      --image-file <base-image.vhdx> \
      --output-image-file ./out/image.iso \
      --output-image-format iso \
      --config-file config.yaml
   ```

   Where:

   - `<base-image.vhdx>`: An official Azure Linux vhdx image file.

## Run LiveOS ISO in Hyper-V

1. Create VM:

   ```powershell
   New-VM -Name mytestvm `
     -MemoryStartupBytes 2GB `
     -Generation 2 `
     -BootDevice CD `
     -NoVHD `
     -SwitchName "Default Switch"
   Set-VMDvdDrive -VMName mytestvm -Path '<image.iso>'
   Set-VMFirmware -VMName mytestvm -SecureBootTemplate MicrosoftUEFICertificateAuthority
   ```

   Where:

   - `<image.iso>`: Is the path of the ISO file that you generated with Image
     Customizer.

2. Start VM:

   ```powershell
   Start-VM -VMName mytestvm
   ```

## Run LiveOS ISO in libvirt/QEMU

1. Create and run VM:

   ```bash
   sudo virt-install \
     --name mytestvm \
     --import \
     --hvm \
     --machine q35 \
     --boot uefi \
     --os-variant linux2024 \
     --disk "<image.iso>,device=cdrom"
   ```

   Where:

   - `<image.iso>`: Is the path of the ISO file that you generated with Image
     Customizer.

     Note: It is sometimes necessary to move the ISO file out of your home directory
     to avoid permissions issues.

## Links

- [LiveOS ISO Support](../concepts/iso.md)
- [--output-image-format CLI parameter](../api/cli.md#--output-image-formatformat)
- [ISO configuration API](../api/configuration/iso.md)
 