---
title: Image Customizer
nav_order: 2
sidebar_position: 2
has_toc: false
---

# Image Customizer

## Overview

The Image Customizer is a tool that can take an existing generic Azure Linux
image and modify it to be suited for particular scenario.

The Image Customizer uses [chroot](https://en.wikipedia.org/wiki/Chroot) (and
loopback block devices) to customize the image. This is the same technology used
to build the Azure Linux images, along with most other Linux distros. This is in
contrast to some other image customization tools, like Packer, which customize
the image by booting it inside a VM.

## Why use Image Customizer?

Unlike VM-based image customization, Image Customizer directly modifies the
image without booting an OS. This 'chroot' based approach has several advantages
and trade-offs:

### Advantages:

- **Lower overhead,** since you don't need to boot up and shutdown the OS.
- **More precision when making changes,** since you won't see any side effects
  that come from the OS running.
- The image has **fewer requirements** (e.g. ssh doesn't need to be installed).

### Limitations:

- **Not all Linux tools play nicely when run under chroot.** For example, while
  it is possible to install Kubernetes using Image Customizer, initialization of
  a Kubernetes cluster node must be done while the OS is running (e.g. using
  cloud-init).

## Supported Hosts

Image Customizer has been tested and verified to work on the following host
environments:

- Ubuntu 22.04
- Azure Linux 2.0 and 3.0
- WSL2 (Windows Subsystem for Linux)

While officially tested on these platforms, Image Customizer will likely work on
other Linux distributions as well.

## Getting Started with Image Customizer

- [Quick Start](./quick-start/quick-start.md) - A beginner-friendly guide to
  quickly customize an Azure Linux image using Image Customizer
- [Things to Avoid](./concepts/things-to-avoid.md) - Best practices to ensure a
  smooth customization experience
- API Documentation:
  - [CLI](./api/cli/cli.md) - Learn about the available command-line interface
    commands for Image Customizer
  - [Configuration](./api/configuration/configuration.md) - Understand how to configure Image
    Customizer to suit your needs

## Telemetry 

Image Customizer collects usage data to help improve the product. This data
helps us understand usage patterns, diagnose issues, improve reliability, and
prioritize new features based on real-world usage. Learn how Image Customizer
collects and uses telemetry, and how to opt out [here](telemetry.md).

## Help and Feedback

If you'd like to report bugs, request features, or contribute to the tool, you
can do so directly through our [GitHub
repo](https://github.com/microsoft/azure-linux-image-tools). We welcome feedback
and contributions from the community!