# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Downloads an OS image from an Azure Storage Account Blob Storage.
#
# Usage:
#   download-image.sh ACCOUNT CONTAINER IMAGE_NAME OUTPUT_DIR
#
#     ACCOUNT: Name of an Azure Storage Account.
#     CONTAINER: Name of a container in the Azure Storage Account.
#     IMAGE_NAME: Image path prefix (i.e. "<DISTRO>/<IMAGE-TYPE>").
#       Image files are expected to be stored at <IMAGE_NAME>/<VERSION>/image.<vhdx|vhd|tar.gz> in <CONTAINER>.
#       For example, "azure-linux/core-efi-vhdx-3.0-amd64/3.0.20250702/image.vhdx".
#       The lexicographically latest <VERSION> will be downloaded.
#       .tar.gz archives are transparently extracted and deleted. It is expected to contain a singe file.
#       For example, "image.vhd.tar.gz" should extract to "image.vhd".
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
    '[.[].name | select(endswith("/image.vhdx") or endswith("/image.vhd") or endswith("/image.vhd.tar.gz"))] | sort | last' \
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

if [[ "$FILENAME" == *.tar.gz ]]; then
    tar -xzf "$OUTPUT_DIR/$FILENAME" -C "$OUTPUT_DIR"
    rm "$OUTPUT_DIR/$FILENAME"
fi
