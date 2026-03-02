#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Validates a base image does not contain packages that tests expect to install.
#
# Usage: validate-image.sh IMAGE_NAME IMAGE_FILE

set -eu

if [ $# -ne 2 ]; then
    echo "Usage: validate-image.sh IMAGE_NAME IMAGE_FILE" >&2
    exit 1
fi

IMAGE_NAME="$1"
IMAGE_FILE="$2"

case "$IMAGE_NAME" in
    azure-linux/core-efi-vhdx-2.0-amd64|\
    azure-linux/core-efi-vhdx-2.0-arm64|\
    azure-linux/core-efi-vhdx-3.0-amd64|\
    azure-linux/core-efi-vhdx-3.0-arm64|\
    azure-linux/core-legacy-vhd-2.0-amd64|\
    azure-linux/core-legacy-vhd-3.0-amd64)
        ROOT_PART=2
        PKG_MGR=rpm
        ;;
    ubuntu/azure-cloud-vhdx-22.04-amd64|\
    ubuntu/azure-cloud-vhdx-22.04-arm64|\
    ubuntu/azure-cloud-vhdx-24.04-amd64|\
    ubuntu/azure-cloud-vhdx-24.04-arm64)
        ROOT_PART=1
        PKG_MGR=dpkg
        ;;
    *)
        echo "ERROR: unrecognized image: '$IMAGE_NAME'" >&2
        exit 1
        ;;
esac

if [ ! -f "$IMAGE_FILE" ]; then
    echo "ERROR: file not found: $IMAGE_FILE" >&2
    exit 1
fi

RAW="$(mktemp --suffix=.raw)"
MNT="$(mktemp -d)"
cleanup() {
    if mountpoint -q "$MNT" 2>/dev/null; then
        umount "$MNT" || true
    fi
    rmdir "$MNT" 2>/dev/null || true

    if [ -n "${LOOP:-}" ]; then
        losetup -d "$LOOP" 2>/dev/null || true
    fi

    rm -f "$RAW"
}
trap cleanup EXIT

qemu-img convert -O raw "$IMAGE_FILE" "$RAW"
LOOP="$(losetup --show -f -P "$RAW")"
mount -o ro "${LOOP}p${ROOT_PART}" "$MNT"

is_installed() {
    if [ "$PKG_MGR" = "dpkg" ]; then
        local status
        status="$(chroot "$MNT" dpkg-query -W -f='${Status}' "$1" 2>/dev/null)" || return 1
        if [ "$status" = "install ok installed" ]; then
            return 0
        fi
        return 1
    else
        chroot "$MNT" rpm -q "$1" >/dev/null 2>&1
    fi
}

FAILED=0
for pkg in tree unzip; do
    if is_installed "$pkg"; then
        echo "FAIL: '$pkg' is pre-installed in $IMAGE_NAME" >&2
        FAILED=1
    else
        echo "OK:   '$pkg' absent"
    fi
done

exit $FAILED
