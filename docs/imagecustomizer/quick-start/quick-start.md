---
title: Quick Start
parent: Image Customizer
nav_order: 1
has_toc: false
---

# Using the Image Customizer Container

The Image Customizer container packages up the Image Customizer executable along with all of its dependencies. This is the *recommended* way to use Image Customizer. 

The container is published to both:

- The [Microsoft Artifact Registry (MCR)](https://mcr.microsoft.com/en-us/artifact/mar/azurelinux/imagecustomizer/) at:

  ```text
  mcr.microsoft.com/azurelinux/imagecustomizer:<version>
  ```

  For example:

  ```text
  mcr.microsoft.com/azurelinux/imagecustomizer:0.17.0
  ```

  You can use the MCR REST API to query available and latest tags:

  ``` bash
  curl -s "https://mcr.microsoft.com/v2/azurelinux/imagecustomizer/tags/list" | jq '.tags[]' 
  ```

- The GitHub Container Registry at:

  ```text
  ghcr.io/microsoft/imagecustomizer:<version>
  ```

  For example:

  ```text
  ghcr.io/microsoft/imagecustomizer:0.17.0
  ```

## Prerequisites

- Linux host
- Docker (or equivalent container engine) installed on your host

## Instructions

1. Download an Azure Linux VHDX image file. 
   - You can [download a marketplace image from Azure](../how-to/download-marketplace-image.md). 
   - You can also download and build one from the [Azure Linux repo](https://github.com/microsoft/azurelinux).

2. Create a customization config file.

   For example:

    ```yaml
    os:
      packages:
        install:
        - dnf
    ```

   For documentation on the supported configuration options, see:
   [Supported configuration](../api/configuration.md)

3. Run the Image Customizer container. Here is a sample command to run it: 

    ```bash
    docker run \
      --rm \
      --privileged=true \
      -v /dev:/dev \
      -v "$HOME/staging:/mnt/staging:z" \
      mcr.microsoft.com/azurelinux/imagecustomizer:0.17.0 \
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

    - `mcr.microsoft.com/azurelinux/imagecustomizer:0.17.0`: The container to run.

    - `imagecustomizer`: Specifies the executable to run within the container.

    Image Customizer options ([CLI API](../api/cli.md)):

    - `--image-file "/mnt/staging/image.vhdx"`: Use the host's `$HOME/staging/image.vhdx`
      file as the input image.

    - `--config-file "/mnt/staging/image-config.yaml"`: Use the host's
      `$HOME/staging/image-config.yaml` file as the config.

    - `--output-image-format`: Output the customized image as a VHDX file.

    - `--output-image-file "/mnt/staging/out/image.vhdx"`: Output the customized image to
      the host's `$HOME/staging/out/image.vhdx` file path.

5. Use the customized image.

   The customized image is placed in the file that you specified with the
   `--output-image-file` parameter. You can now use this image as you see fit.
   (For example, boot it in a Hyper-V VM.)

## Helper script

`run-container.sh`

This script wraps the Docker call. It is intended to make using the Image Customizer
container a little easier.

For example, this is the equivalent call to the above example:

```bash
run-container.sh \
    -t mcr.microsoft.com/azurelinux/imagecustomizer:0.17.0 \
    -i "$HOME/staging/image.vhdx" \
    -c "$HOME/staging/image-config.yaml" \
    -f vhdx \
    -o "$HOME/staging/out/image.vhdx" \
```

## Next Steps

- Learn how to [deploy the customized image as an Azure VM](../how-to/azure-vm.md)
- Learn more about the [Image Customizer command line](../api/cli.md)
- Learn more about the [Image Customizer config options](../api/configuration.md)