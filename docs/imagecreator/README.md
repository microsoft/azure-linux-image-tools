---
title: Image Creator
nav_order: 3
has_toc: false
---

# Image Creator

## Overview

Image Creator is a tool for building Azure Linux operating system images from scratch.

## What is Image Creator?

Image Creator generates a minimal "seed" image, which serves as a base for further customization.
This seed image can then be customized using the Image Customizer tool to apply additional
configurations and tailor the image to your specific needs. For advanced customizations, it is
recommended to use Image Customizer on top of the seed image produced by Image Creator.

> **Note:** Image Creator does not validate whether the generated image will be bootable. For best
> results, it is recommended to use Image Customizer on top of official Azure Linux images that are
> published using Image Creator.

## Getting Started with Image Creator

- [Quick Start](./quick-start/quick-start-binary.md) - A beginner-friendly guide to
  quickly build an Azure Linux image using Image Creator
- API Documentation:
  - [CLI](./api/cli.md) - Learn about the available command-line interface
    commands for Image Creator
  - [Configuration](./api/configuration.md) - Understand how to configure Image
    Creator to suit your needs

## Help and Feedback

If you'd like to report bugs, request features, or contribute to the tool, you
can do so directly through our [GitHub
repo](https://github.com/microsoft/azure-linux-image-tools). We welcome feedback
and contributions from the community!

