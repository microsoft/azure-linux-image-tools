#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Validates that a downloaded base image does not contain packages that the
# tests expect to install (e.g. jq, tree). Called from the CI workflow after
# images are downloaded.
#
# The root partition number and files to check are determined from IMAGE_NAME.
# If IMAGE_NAME is not recognized, the script fails with an error so that new
# images are not silently skipped.
#
# The image may be a VHDX or VHD, so it must be converted to raw before losetup
# can parse its partition table. The raw copy is deleted after validation.
#
# Usage:
#   validate-image.sh IMAGE_NAME IMAGE_FILE
#
#     IMAGE_NAME: The image name passed to download-image.sh
#       (e.g. "azure-linux/core-efi-vhdx-3.0-amd64").
#     IMAGE_FILE: The path to the downloaded image file
#       (e.g. "/path/to/output/image.vhdx").

set -eu

if [ $# -ne 2 ]; then
    echo "Usage: validate-image.sh IMAGE_NAME IMAGE_FILE" >&2
    exit 1
fi

IMAGE_NAME="$1"
IMAGE_FILE="$2"

# Determine root partition number and files to check based on IMAGE_NAME.
case "$IMAGE_NAME" in
    azure-linux/core-efi-vhdx-2.0-amd64|\
    azure-linux/core-efi-vhdx-2.0-arm64|\
    azure-linux/core-efi-vhdx-3.0-amd64|\
    azure-linux/core-efi-vhdx-3.0-arm64|\
    azure-linux/core-legacy-vhd-2.0-amd64|\
    azure-linux/core-legacy-vhd-3.0-amd64)
        ROOT_PARTITION_NUM=2
        FILES_TO_CHECK=(/usr/bin/jq /usr/bin/tree)
        ;;
    ubuntu/azure-cloud-vhdx-22.04-amd64|\
    ubuntu/azure-cloud-vhdx-22.04-arm64|\
    ubuntu/azure-cloud-vhdx-24.04-amd64|\
    ubuntu/azure-cloud-vhdx-24.04-arm64)
        ROOT_PARTITION_NUM=1
        FILES_TO_CHECK=(/usr/bin/jq /usr/bin/tree)
        ;;
    *)
        echo "ERROR: image '$IMAGE_NAME' cannot be validated: unrecognized image name." >&2
        echo "  Add a case for this image in validate-image.sh." >&2
        exit 1
        ;;
esac

if [ ! -f "$IMAGE_FILE" ]; then
    echo "ERROR: image file not found: $IMAGE_FILE" >&2
    exit 1
fi

RAW_IMAGE="$(mktemp --suffix=.raw)"
MOUNT_DIR="$(mktemp -d)"

cleanup() {
    if mountpoint -q "$MOUNT_DIR" 2>/dev/null; then
        umount "$MOUNT_DIR" || true
    fi
    rmdir "$MOUNT_DIR" 2>/dev/null || true

    if [ -n "${LOOP_DEV:-}" ]; then
        losetup -d "$LOOP_DEV" 2>/dev/null || true
    fi

    rm -f "$RAW_IMAGE"
}
trap cleanup EXIT

echo "Validating $IMAGE_FILE (image: $IMAGE_NAME)..."

echo "Converting to raw..."
qemu-img convert -O raw "$IMAGE_FILE" "$RAW_IMAGE"

echo "Setting up loopback device..."
LOOP_DEV="$(losetup --show -f -P "$RAW_IMAGE")"
echo "Loopback device: $LOOP_DEV"

PART_DEV="${LOOP_DEV}p${ROOT_PARTITION_NUM}"
echo "Mounting root partition: $PART_DEV"
mount -o ro "$PART_DEV" "$MOUNT_DIR"

FAILED=0
for FILE_PATH in "${FILES_TO_CHECK[@]}"; do
    FULL_PATH="${MOUNT_DIR}${FILE_PATH}"
    if [ -e "$FULL_PATH" ]; then
        echo "ERROR: $FILE_PATH already exists in the base image." >&2
        echo "  Tests expect to install this package, so it must not be pre-installed." >&2
        FAILED=1
    else
        echo "OK: $FILE_PATH is absent (as expected)."
    fi
done

if [ "$FAILED" -ne 0 ]; then
    echo "Validation failed: $IMAGE_NAME contains packages that tests expect to install." >&2
    exit 1
fi

echo "Validation passed: $IMAGE_NAME"
