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
   DISK_RG="<disk-rg>"

   az disk create -g "$DISK_RG" -n "$DISK_NAME" --image-reference "$IMAGE_URN"
   ```

7. 
