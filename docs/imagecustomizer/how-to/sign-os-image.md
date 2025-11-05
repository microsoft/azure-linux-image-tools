---
title: Sign OS Image
parent: How To
nav_order: 9
---

# Sign an OS Image and then run it in libvirt/QEMU

This is a guide on how to use Image Customizer to sign an OS image using your own
certificates and then run that OS image under libvirt/QEMU.
The OS image uses UKI and usr dm-verity.

## Words of caution

This is intended as an example only.

Managing private keys securely is a non-trivial task.
In general, a private key should not be accessible to anything or anyone except a secure
signing service (e.g. Azure Key Vault).

## Steps

1. Install the following tools:

   TODO

2. Create a directory to stage all the build artifacts:

   ```bash
   STAGE_DIR="<staging-directory>"
   mkdir -p "$STAGE_DIR"
   ```

   Where:

   - `<staging-directory>` is the absolute path of the directory where you will store
     all the build artifacts.

3. Create a file named `$STAGE_DIR/image-config.yaml` with the following
   contents:

   ```yaml
   ```

4. Run Image Customizer to create the new image:

   ```bash
   IMG_CUSTOMIZER_TAG="ghcr.io/microsoft/imagecustomizer:1.1.0"
   docker run \
     --rm \
     --privileged=true \
     -v /dev:/dev \
     -v "$STAGE_DIR:/mnt/staging:z" \
     "$IMG_CUSTOMIZER_TAG" \
       --image-file "/mnt/staging/image.vhd" \
       --config-file "/mnt/staging/image-config.yaml" \
       --build-dir "/mnt/staging/build" \
       --output-image-format "qcow2" \
       --output-image-file "/mnt/staging/out/image.qcow2" \
       --log-level debug
   ```

5. 
