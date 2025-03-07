---
title: Download Marketplace Image
parent: How To
nav_order: 6
---

# Download Azure Linux Marketplace Image

This is a guide on how to download a marketplace image from Azure so that it can be
customized using Image Customizer.

## Steps

1. Login to Azure with the CLI.

   ```bash
   az login
   ```

2. Verify your default subscription.

   ```bash
   az account show --output table
   ```

3. If necessary, change your default subscription to one you have permissions to create
   resources in.

   ```bash
   SUBSCRIPTION_NAME="<subscription-name>"
   az account set --subscription "$SUBSCRIPTION_NAME"
   ```

4. List the Azure Linux marketplace images.

   For Azure Linux 2.0:

   ```bash
   az vm image list --publisher MicrosoftCBLMariner --offer cbl-mariner --sku cbl-mariner-2-gen2 --all --output table
   ```

   For Azure Linux 3.0:

   ```bash
   az vm image list --publisher MicrosoftCBLMariner --offer azure-linux-3 --sku azure-linux-3-gen2 --all --output table
   ```

5. Pick an image and copy its URN.

   For example:

   ```bash
   IMAGE_URN="MicrosoftCBLMariner:azure-linux-3:azure-linux-3-gen2:3.20250102.02"
   ```

6. Create a managed disk from the marketplace image.

   ```bash
   DISK_NAME="<disk-name>"
   DISK_RG="<disk-resource-group-name>"
   DISK_LOC="<azure-location>"

   az group create --location "$DISK_LOC" --name "$DISK_RG"
   az disk create -g "$DISK_RG" -n "$DISK_NAME" --image-reference "$IMAGE_URN"
   ```

7. Generate SAS URL:

   ```bash
   SAS_JSON="$(az disk grant-access --duration-in-seconds 86400 --access-level Read --name "$DISK_NAME" --resource-group "$DISK_RG")"
   SAS_URL="$(jq -r '.accessSas // .accessSAS' <<< "$SAS_JSON")"
   ```

8. Download VHD:

   ```bash
   az storage blob download -f ./image.vhd --blob-url "$SAS_URL"
   ```

9. Delete temporary resources:

   ```bash
   az group delete --name "$DISK_RG" --no-wait
   ```

## Helpful links

- [Download a Linux VHD from Azure](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/download-vhd?tabs=azure-cli)
