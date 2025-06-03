---
parent: Concepts
title: PXE Support
nav_order: 3
---

# Image Customizer PXE Support

## Overview

Booting a host with an OS served over the network is one of the most popular
methods for booting baremetal hosts. It requires no physical access to individual
hosts and also centralizes the deployment configuration to a single server.

One way of enabling such setup is using the PXE (Preboot eXecution Environment)
Boot protocol. The user can setup a server with all the OS artifacts, a DHCP
endpoint, and a tftp connection endpoint. When a client machine is powered on,
its firmware will look for a DHCP server on the same network and will find the
one configured by the user.

The DHCP server will serve information about the tftp endpoint to the client,
and the client firmware can then proceed with retrieving the OS artifacts over
tftp, loading them into memory, and finally handing control over to the
loaded OS.

The tftp protocol expects certain artifacts to be present on the server:

- the boot loader (the shim and something like grub).
- the boot loader configuration (like grub.cfg).
- the kernel image.
- the initrd image.

Once retrieved, the boot loader is run. Then the boot loader reads the
boot loader configuration and then transfers control over to the kernel image
with the retrieved initrd image as its file system.

The initrd image is customized to perform the next set of tasks now that an
OS is running. The tasks can range from just running some local scripts all
the way to installing another OS.

## Live OS PXE Support

A Live OS is when a system is booted from removable media (like an ISO or a USB)
or from the network (like using PXE).In such flows, the OS file system is not
installed to persistent storage on the host prior to booting the system.

The full OS file system can either be embedded in the initrd image itself
(`initramfsType=full-os`) or be stored in a separate image that will be
bootstrapped by the initrd image (`initramfsType=bootstrap`).

The main difference is the size of the generated initrd image:
- When embedded, the initrd image size is larger and may not meet the memory
  restriction on certain hardware skus (leading to an "out of memory" grub
  error). However, this is much simpler to deploy to a PXE server since the
  full OS content will be served using the PXE protocol without any extra setup.
- When bootstrapped, the initrd image size will be much smaller (~30MB) - which
  solves the memory restriction issue on affected hardware models. The downside
  is that extra PXE environment setup is necessary (like setting up a server to
  host the bootstrapped extra file and server it).

The **Image Customizer** can produce the Live OS artifacts necessary to PXE boot
a host. The artifacts include:

- the boot loader (the shim and something like grub)
- the boot loader configuration
- the kernel image
- the initrd image
- the rootfs image (required only if `initramfsType=bootstrap`)
- other user defined artifacts (optional)

Dracut enables the PXE bootstrap flow through the use of the `livenet` module -
where it inspects the `root=live:liveos-iso-url` kernel parameter from the boot
loader config file, and if it recognizes the `liveos-iso-url` protocol, it downloads
the bootstrapped full OS image, and then proceeds to pivot to the embedded OS file system
image.

The user can customize the full OS image using the Image Customizer as
usual.

The bootstrapped full OS image is a bootable ISO containing the same
configuration (same bootloader configuration, same kernel, same full OS file
system, etc.). This can be very handy when testing the PXE configuration
without having to setup a PXE environment.

In case the user needs to download additional artifacts, the user can write
a daemon on the full OS file system which will:
- run when control is transferred to the full OS.
- download any additional items.

## Creating and Deploying PXE Boot Artifacts

The Image Customizer can be told to create the PXE artifacts by simply setting
the output format to `pxe` on the command-line. The user tell the Image Customizer
to output the artifacts to a folder or as a tar.gz archive.

For additional details, see the [PXE Configuration](./configuration/pxe.md) page.

Below is a list of required artifacts and where on the PXE server they should
be deployed:

```
ISO media layout           artifacts local folder      target on PXE server
-----------------------    ------------------------    ------------------------------
|- efi                      |                           <tftp-server-root>
   |- boot                  |                             |
      |- bootx64.efi        |- bootx64.efi                |- bootx64.efi
      |- grubx64.efi        |- grubx64.efi                |- grubx64.efi
|- boot                     |- boot                       |- boot
   |- grub2                    |- grub2                      |- grub2
      |- grub-pxe.cfg             |- grub.cfg                   |- grub.cfg
      |- grubenv                  |- grubenv                    |- grubenv
      |- grub.cfg
   |- vmlinuz                  |- vmlinuz                    |- vmlinuz
   |- initrd.img               |- initrd.img                 |- initrd.img

                                                        <yyyy-server-root>
|- other-user-artifacts     |- other-user-artifacts       |- other-user-artifacts
                            |- image.iso               |- image.iso
```

Notes:

- Note that the `/boot/grub2/grub.cfg` file in the ISO media is not used for
  PXE booting. Instead, the `/boot/grub2/grub-pxe.cfg` gets renamed to `grub.cfg`
  and is used instead.
- `yyyy` can be any protocol supported by Dracut's `livenet` module (i.e
  tftp, http, etc).
- `image.iso` is the bootstrapped image containing the full OS file system. It
  is generated by the Image Customizer when the output format is set to `pxe`.
- The bootstrapped ISO image file location under the server root is customizable -
  but it must be such that its URL matches what is specified in the grub.cfg
  `root=live:<URL>`.
