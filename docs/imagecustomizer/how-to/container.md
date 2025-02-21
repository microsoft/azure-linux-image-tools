---
title: Use Container
parent: How To
nav_order: 5
---

# Using the Image Customizer Container

The Image Customizer container packages up the Image Customizer executable along with
all of its dependencies.

The container is available at:

```text
mcr.microsoft.com/azurelinux/imagecustomizer:<version>
```

For example:

```text
mcr.microsoft.com/azurelinux/imagecustomizer:0.12.0
```

## Prerequisites

Unlike running the executable directly, the only prerequisites needed is a Linux host
and Docker (or equivalent container engine).

## Example

```bash
docker run \
  --rm \
  --privileged=true \
  -v /dev:/dev \
  -v "$HOME/staging:/mnt/staging:z" \
  mcr.microsoft.com/azurelinux/imagecustomizer:0.12.0 \
  imagecustomizer \
    --image-file "/mnt/staging/image.vhdx" \
    --config-file "/mnt/staging/image-config.yaml" \
    --build-dir "/build" \
    --output-image-format "vhdx" \
    --output-image-file "/mnt/staging/out/image.vhdx"
```

Docker options:

- `run --rm`: Runs the container and cleans up the container once the program
  has completed.

- `--privileged=true`: Gives the container root permissions, which is needed to mount
  loopback devices (i.e. disk files) and partitions.

- `-v /dev:/dev`: When mounting loopback devices, the container needs the partition
  device nodes to be populated under `/dev`. But the udevd service runs in the host not
  the container. So, the container doesn't receive udev updates.

  This option maps in the host's version of `/dev` into the container, instead of the
  container getting its own `/dev`.

- `-v $HOME/staging:/mnt/staging:z`: Mounts a host directory (`$HOME/staging`) into the
   container. This can be used to easily pass files in and out of the container.

- `mcr.microsoft.com/azurelinux/imagecustomizer:0.12.0`: The container to run.

- `imagecustomizer`: Specifies the executable to run within the container.

Image Customizer options ([CLI API](../api/cli.md)):

- `--image-file "/mnt/staging/image.vhdx"`: Use the host's `$HOME/staging/image.vhdx`
  file as the input image.

- `--config-file "/mnt/staging/image-config.yaml"`: Use the host's
  `$HOME/staging/image-config.yaml` file as the config.

- `--output-image-format`: Output the customized image as a VHDX file.

- `--output-image-file "/mnt/staging/out/image.vhdx"`: Output the customized image to
  the host's `$HOME/staging/out/image.vhdx` file path.

## Helper script

[run-mic-container.sh](https://github.com/microsoft/azure-linux-image-tools/blob/stable/toolkit/tools/imagecustomizer/container/run-mic-container.sh)

This script wraps the Docker call. It is intended to make using the Image Customizer
container a little easier.

For example, this is the equivalent call to the above example:

```bash
run-mic-container.sh \
    -t mcr.microsoft.com/azurelinux/imagecustomizer:0.12.0 \
    -i "$HOME/staging/image.vhdx" \
    -c "$HOME/staging/image-config.yaml" \
    -f vhdx \
    -o "$HOME/staging/out/image.vhdx" \
```
