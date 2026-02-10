---
parent: Concepts
title: Secure OS images
nav_order: 8
---

# Creating secure OS images

Image Customizer has support for a couple of features that can help make a Linux OS
image more resistant to malware.

These two features are:

- UKI
- Verity

## UKI

A UKI (Unified Kernel Image) is an EFI file that bonds the kernel binary, the kernel
command-line args, and the initramfs into a single file. Using a signed UKI helps
protect the OS boot process from malicious modification.

## Verity

dm-verity is a feature that uses cryptography to make a partition immutable. Such
partitions can't be modified without detection.

There are two forms of dm-verity that Image Customizer supports:

1. Root verity.

   This makes the entire root partition (`/`) read-only. In such setups, typically only
   the `/var` directory is mounted on a read-write data partition.

   This option produces a more secure image than usr verity. However, it is also more
   difficult to use since a lot of Linux compatible software is not designed to handle
   most directories being immutable and read-only, particularly the `/etc` directory.

2. Usr verity.

   This mounts `/usr` on its partition and makes it read-only.