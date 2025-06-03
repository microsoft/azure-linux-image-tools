# Live OS

A Live OS is when a system is booted from removable media (for example, an ISO
or a USB) or from the network (for example, using PXE). In such flows, the OS
file system is not installed to persistent storage on the host prior to fully
booting the system.

Image Customizer can create Live OS for both scenarios:
- The ISO scenario (an ISO image).
- The PXE scenario (artifacts folder or tar.gz).

The Live OS is typically comprised of the following core artifacts:
- the boot loader (the shim and something like grub)
- the boot loader configuration
- the kernel image
- the initrd image
- additional files (optional)

## Initramfs Contents

The user can decide whether the full OS file system will be embedded in the
initrd image itself or stored in a separate image (in which case, a process
on the initrd image will need to find it, and pivot to it).

The main difference between the two configurations is:
- When the initrd image is the full OS:
  - the initrd image size is larger and may not meet the memory restriction on
    certain hardware skus (leading to an "out of memory" grub error). However,
    this is much simpler to deploy to a PXE server since the full OS content
    will be served using the PXE protocol without any extra setup.
  - initrd is customized using the Image Customizer configuration constructs
    directly (install packages, copy files, etc).
  - selinux is not supported (mainly because CPIO does not support extended file
    system attributes).
- When the initrd image is a bootstrap image:
  - the initrd image size will be much smaller (~30MB) - which solves the memory
    restriction issue on affected hardware models.
  - The downside is that extra PXE environment setup is necessary (like setting up
    a server to host the bootstrapped extra file and server it).
  - initrd is customized using Dracut configuration files. The user will need to
    craft those files, use Image Customizers to place them in the right locations
    on the root file system so that when Dracut is run to generate a new initrd,
    it will take them into consideration.
  - selinux can be enabled.

The user can specify either configuration using the `initramfsType` property.

## Bootstrap Implementation

- ISO support:
  - The bootstrap support is implemented using Dracut's `dmsquash` module. The
    module expects the full OS image to be a squashfs image placed under
    `/liveos/rootfs.img`. Once it is found, it will be mounted, an overlay will
    be created, and then the kernel will pivot to it.
- PXE Support:
  - To support the Live OS bootstrap PXE scenario, Dracut's `livenet` module
    downloads the bootstrapped image (expects an .ISO), and then passes it to the
    `dmsquash` module. To enable this Dracut flow, the kernel command-line must
    contain a `root` parameter on the form `root=live:liveos-iso-url`.
