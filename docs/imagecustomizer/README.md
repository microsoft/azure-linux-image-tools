---
title: Image Customizer
nav_order: 2
---

# Image Customizer

## Overview

The Image Customizer is a tool that can take an existing generic Azure Linux
image and modify it to be suited for particular scenario.

The Image Customizer uses [chroot](https://en.wikipedia.org/wiki/Chroot) (and loopback
block devices) to customize the image.
This is the same technology used to build the Azure Linux images, along with most other
Linux distros.
This is in contrast to some other image customization tools, like Packer, which
customize the image by booting it inside a VM.

There are a number of advantages and disadvantages to the `chroot` approach to
customizing images.

Advantages:

- Lower overhead, since you don't need to boot up and shutdown the OS.
- More precision when making changes, since you won't see any side effects that come
  from the OS running.
- The image has fewer requirements (e.g. ssh doesn't need to be installed).

Disadvantages:

- Not all Linux tools play nicely when run under chroot.
  For example, while it is possible to install Kubernetes using Image Customizer,
  initialization of a Kubernetes cluster node must be done while the OS is running
  (e.g. using cloud-init).

## Helpful Links

- [Quick Start](./how-to/quick-start.md)
- [Things to Avoid](./concepts/things-to-avoid.md)
- API:
  - [CLI](./api/cli.md)
  - [Configuration](./api/configuration.md)
