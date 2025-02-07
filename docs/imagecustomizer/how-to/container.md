---
title: Use Container
parent: How To
nav_order: 5
---

# Using the Image Customizer Container

The Image Customizer container is designed to simplify the process of
customizing and configuring system images using the Image Customizer tool.

## Running the Container

To use the Image Customizer container, you will need to pass several parameters
and mount appropriate volumes for the image and device access. Below is a
step-by-step guide on how to run this container:

### Prepare Your Environment

Ensure that your configuration file (config.yaml) is ready and accessible. This
file should define the customization parameters for the MIC tool. Details please
see [configuration](../api/configuration.md).

### Run the Container

Pull the image:

```bash
docker pull mcr.microsoft.com/azurelinux/imagecustomizer:0.7.0
```

You can use your own base image, for example:

```bash
docker run --rm --privileged=true \
   -v ~/image:/image:z \
   -v /dev:/dev \
   mcr.microsoft.com/azurelinux/imagecustomizer:0.3.0 \
   --image-file /baseimg.vhdx \
   --config-file /config.yaml \
   --output-image-format raw \
   --output-image-file /image/customized.raw
```

Alternatively, you can use the
[run.sh](https://github.com/microsoft/azure-linux-image-tools/blob/stable/toolkit/tools/imagecustomizer/container/run.sh)
script on the container which runs `imagecustomizer` with a base image downloaded from
MCR.

Usage: `run.sh $version_tag`

For a complete usage example, refer to
[test-mic-container.sh](https://github.com/microsoft/azure-linux-image-tools/blob/stable/toolkit/tools/imagecustomizer/container/test-mic-container.sh).

### Check the Output

After the container executes, check the output directory on your host for the
customized image file. This file contains your customized system image.
