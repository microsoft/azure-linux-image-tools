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

A Live OS is when a system is booted from an ISO or downloaded through something
like the PXE boot process. In such flow, the OS is running, but nothing has been
installed to the host.

The OS file system can be either embedded in the initrd image itself (`initramfsType=full-os`)
or stored in a separate (bootstrap) image (`initramfsType=bootstrap`).

The main difference is the size of the generated initrd image:
- When embedded, the initrd image size is larger and may not meet the memory
  restriction on certain hardware skus (leading to an "out of memory" grub
  error). However, this is much simpler to deploy to a PXE server since the
  full OS content will be served using the PXE protocol without any extra setup.
- When bootstrapped, the initrd image size will be much smaller (~30MB) - which
  solves the memory restriction issue on affected hardware models. The downside
  is that extra PXE environment setup is necessary (like setting up a server to
  host the bootstrapped extra file and server it).

It is worth noting that the bootstrap support is implemented using Dracut's
`dmsquash-live` module.

The **Image Customizer** produces such [LiveOS ISO](./iso.md) images. A typical
image holds the following artifacts:

- the boot loader (the shim and something like grub)
- the boot loader configuration
- the kernel image
- the initrd image
- the rootfs image (required only if `initramfsType=bootstrap`)
- other user defined artifacts (optional)

Dracut enables that entire flow through the use of the `livenet` module - where
it inspects the `root=live:liveos-iso-url` kernel parameter from the boot loader
config file, and if it recognizes the `liveos-iso-url` protocol, it downloads
the ISO, and then proceeds to pivot to the embedded rootfs image.

The user can customize the rootfs using the Image Customizer as
usual. In case of additional artifacts that need downloading, the user can
install a daemon on the rootfs which will run when control is transferred to
the rootfs image and download any additional items.

## Creating and Deploying PXE Boot Artifacts

The Image Customizer produces [LiveOS ISO](./iso.md) images that are also PXE
bootable. So, the user can simply create an ISO image as usual, and the output
can be taken and deployed to a PXE server.

To make the deployment of the generated artifacts easier for the user, the
Image Customizer offers the following configurations:

- In the input configuration, there is a `pxe` node under which the user can
  configure PXE related properties - like the URL of the LiveOS ISO image to
  download (note that this image is the same image being built).
  See the [Image Customizer configuration](../api/configuration/pxe.md)
  page for more information.
- When invoking the Image Customizer, the user can also elect to
  export the artifacts to a local folder.
  See the [Image Customizer command line](../api/cli.md#--output-pxe-artifacts-dir)
  page for more information.

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
                            |- <liveos>.iso               |- <liveos>.iso
```

Notes:

- Note that the `/boot/grub2/grub.cfg` file in the ISO media is not used for
  PXE booting. Instead, the `/boot/grub2/grub-pxe.cfg` gets renamed to `grub.cfg`
  and is used instead.
- `yyyy` can be any protocol supported by Dracut's `livenet` module (i.e
  tftp, http, etc).
- The ISO image file location under the server root is customizable -
  but it must be such that its URL matches what is specified in the grub.cfg
  `root=live:<URL>`.
- While the core OS artifacts (the bootloader, its configuration, the kernel,
  initrd image, and rootfs image) will be downloaded and used automatically,
  the user will need to independently implement a way to download any
  additional artifacts. For example, the user can implement a daemon (and place
  it on the root file system) that will reach out and download the additional
  artifacts when it is up and running. The daemon can be configured with where
  to download the artifacts from, and what to do with them.
