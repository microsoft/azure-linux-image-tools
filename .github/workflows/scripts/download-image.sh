# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Downloads an OS image from an Azure Storage Account Blob Storage.
#
# Usage:
#   download-image.sh ACCOUNT CONTAINER IMAGE_NAME OUTPUT_DIR
#
#     ACCOUNT: Name of an Azure Storage Account.
#     CONTAINER: Name of a container in the Azure Storage Account.
#     IMAGE_NAME: Name of the image.
#       Image files are expected to be stored at "<IMAGE_NAME>/<VERSION>/image.<vhdx|vhd>" within the blob container.
#       IMAGE_NAME typically has the format "<DISTRO>/<IMAGE-TYPE>".
#       For example, "azure-linux/core-efi-vhdx-3.0-amd64/3.0.20250702/image.vhdx".
#     OUTPUT_DIR: The directory to output files to.

set -eu

ACCOUNT="$1"
CONTAINER="$2"
IMAGE_NAME="$3"
OUTPUT_DIR="$4"

mkdir -p "$OUTPUT_DIR"

CONTAINERS_JSON=$(
    az storage blob list \
        --auth-mode login \
        --account-name "$ACCOUNT" \
        --container-name "$CONTAINER" \
        --prefix "$IMAGE_NAME/"
)

LATEST_IMAGE=$(
    jq \
    -r \
    --arg image "$IMAGE_NAME" \
    '[.[].name | select(endswith("/image.vhdx") or endswith("/image.vhd"))] | sort | last' \
    <<< "$CONTAINERS_JSON" \
)

echo "Latest: $LATEST_IMAGE"

FILENAME="$(basename "$LATEST_IMAGE")"

az storage blob download \
    --auth-mode login \
    --account-name "$ACCOUNT" \
    --container-name "$CONTAINER" \
    --name "$LATEST_IMAGE" \
    --file "$OUTPUT_DIR/$FILENAME" \
    --output none
