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
#       Image files are expected to be stored at "<IMAGE_NAME>/<VERSION>/image.vhdx" within the blob container.
#       IMAGE_NAME typically has the format "<DISTRO>/<IMAGE-TYPE>".
#       For example, "azure-linux/core-efi-vhdx-3.0-amd64/3.0.20250702/image.vhdx".
#     OUTPUT_DIR: The directory to output files to.
#     VERSION: Optional pinned version. When set, the image is downloaded from
#       "<IMAGE_NAME>/<VERSION>/image.vhdx" (or .vhd) directly without listing
#       blobs. When omitted, the lex-last blob under "<IMAGE_NAME>/" is picked.

set -eu

ACCOUNT="$1"
CONTAINER="$2"
IMAGE_NAME="$3"
OUTPUT_DIR="$4"
VERSION="${5:-}"

mkdir -p "$OUTPUT_DIR"

if [[ -n "$VERSION" ]]; then
    # Pinned version: try .vhdx first, fall back to .vhd. Use blob list
    # restricted to the pinned version's directory so we don't have to guess
    # the extension and so we still get a clear error if neither exists.
    CONTAINERS_JSON=$(
        az storage blob list \
            --auth-mode login \
            --account-name "$ACCOUNT" \
            --container-name "$CONTAINER" \
            --prefix "$IMAGE_NAME/$VERSION/"
    )

    BLOB=$(
        jq \
        -r \
        '[.[].name | select(endswith("/image.vhdx") or endswith("/image.vhd"))] | sort | last' \
        <<< "$CONTAINERS_JSON" \
    )

    if [[ -z "$BLOB" || "$BLOB" == "null" ]]; then
        echo "ERROR: no image.vhdx or image.vhd found at $IMAGE_NAME/$VERSION/" >&2
        exit 1
    fi

    echo "Pinned: $BLOB"
else
    CONTAINERS_JSON=$(
        az storage blob list \
            --auth-mode login \
            --account-name "$ACCOUNT" \
            --container-name "$CONTAINER" \
            --prefix "$IMAGE_NAME/"
    )

    BLOB=$(
        jq \
        -r \
        '[.[].name | select(endswith("/image.vhdx") or endswith("/image.vhd"))] | sort | last' \
        <<< "$CONTAINERS_JSON" \
    )

    if [[ -z "$BLOB" || "$BLOB" == "null" ]]; then
        echo "ERROR: no image.vhdx or image.vhd found at $IMAGE_NAME/" >&2
        exit 1
    fi

    echo "Latest: $BLOB"
fi

FILENAME="$(basename "$BLOB")"

az storage blob download \
    --auth-mode login \
    --account-name "$ACCOUNT" \
    --container-name "$CONTAINER" \
    --name "$BLOB" \
    --file "$OUTPUT_DIR/$FILENAME" \
    --output none
