# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Downloads an OS image from an Azure Stroage Account Blob Storage.
#
# Usage:
#   download-image.sh ACCOUNT CONTAINER IMAGE_NAME OUTPUT_DIR
#
#     ACCOUNT: Name of an Azure Storage Account.
#     CONTAINER: Name of a container in the Azure Storage Account.
#     IMAGE_NAME: Name of the image type. Images are expected to be named: '<IMAGE-TYPE>/<VERSION>/image.vhdx".
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

LATEST_DIR=$(
    jq \
    -r \
    --arg image "$IMAGE_NAME" \
    '[.[].name | select(endswith("/image.vhdx")) | rtrimstr("/image.vhdx")] | sort | last' \
    <<< "$CONTAINERS_JSON" \
)

echo "Latest: $LATEST_DIR"

az storage blob download \
    --auth-mode login \
    --account-name "$ACCOUNT" \
    --container-name "$CONTAINER" \
    --name "$LATEST_DIR/image.vhdx" \
    --file "$OUTPUT_DIR/image.vhdx" \
    --output none
