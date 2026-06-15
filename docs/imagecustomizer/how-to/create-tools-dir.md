---
title: Create Tools Directory
parent: How To
nav_order: 9
has_toc: false
---

# Creating the Tools Directory

The `--tools-dir` flag used by the Image Customizer
[create](../api/cli/create.md) and [customize](../api/cli/customize.md) subcommands
requires a directory that contains a package manager (tdnf or dnf) and its runtime
dependencies.

This directory is used as a bootstrap environment to install packages into the target
image. It is not added to the image itself.

## Prerequisites

- Linux host
- Docker (or equivalent container engine) installed

## How It Works

The tools directory is created by exporting the filesystem of a container image that
already contains the package manager you need. Image Customizer then uses this directory
as a chroot environment, mounting the target image inside it at `/_imageroot` and
running `tdnf --installroot=/_imageroot`.

## Instructions

### Azure Linux 3.0

The Azure Linux base core container includes tdnf and all its dependencies.

```bash
# Pull the container image
docker pull mcr.microsoft.com/azurelinux/base/core:3.0

# Create a temporary container (does not start it)
docker create --name ic-tools-azurelinux-3.0 mcr.microsoft.com/azurelinux/base/core:3.0

# Export the container filesystem and extract it into a directory
mkdir -p "$HOME/staging/tools-azurelinux-3.0"
docker export ic-tools-azurelinux-3.0 | tar -x -C "$HOME/staging/tools-azurelinux-3.0"

# Remove the temporary container
docker rm ic-tools-azurelinux-3.0
```

The tools directory is now at `$HOME/staging/tools-azurelinux-3.0`.

### Fedora 42

The Fedora base container includes dnf and all its dependencies.

```bash
# Pull the container image
docker pull quay.io/fedora/fedora:42

# Create a temporary container (does not start it)
docker create --name ic-tools-fedora-42 quay.io/fedora/fedora:42

# Export the container filesystem and extract it into a directory
mkdir -p "$HOME/staging/tools-fedora-42"
docker export ic-tools-fedora-42 | tar -x -C "$HOME/staging/tools-fedora-42"

# Remove the temporary container
docker rm ic-tools-fedora-42
```

The tools directory is now at `$HOME/staging/tools-fedora-42`.

## Reusing the Tools Directory

The tools directory can be reused across multiple Image Customizer runs as long as the
container image it was created from has not changed. You do not need to recreate it each
time.

To pick up updates (e.g. a newer version of tdnf), pull the container image again and
repeat the steps above to recreate the directory.

## Using the Tools Directory

Pass the tools directory to Image Customizer with the `--tools-dir` flag.

For example, when running Image Customizer via Docker:

```bash
docker run \
  --rm \
  --privileged=true \
  -v /dev:/dev \
  -v "$HOME/staging:/mnt/staging:z" \
  mcr.microsoft.com/azurelinux/imagecustomizer:latest customize \
    --tools-dir /mnt/staging/tools-azurelinux-3.0 \
    ...
```

Note that `$HOME/staging` is mounted into the container at `/mnt/staging`, so the tools
directory path inside the container is `/mnt/staging/tools-azurelinux-3.0`.

## See Also

- [create subcommand](../api/cli/create.md)
- [customize subcommand](../api/cli/customize.md)
- [Creating a new image from scratch](../how-to/create-image.md)
