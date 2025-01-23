---
title: Create Azure VM
parent: How To
nav_order: 3
---

# Create a customized image and deploy it as an Azure VM

This guide shows you how to customize a marketplace Azure Linux image and then deploy it
to Azure as a VM.

## Steps

1. Create a directory to stage all the build artifacts:

   ```bash
   STAGE_DIR="<staging-directory>"
   mkdir -p "$STAGE_DIR"
   ```

2. Download Azure Linux VHD file:
   [Download Azure Linux Marketplace Image](./download-marketplace-image.md)

3. Move the downloaded VHD file to the staging directory.

   ```bash
   mv ./image.vhd "$STAGE_DIR"
   ```

4. Create a Python script containing a HTTP service.

   Create a file named `$STAGE_DIR/myservice.py` with the following contents:

   ```python3
   
   ```

