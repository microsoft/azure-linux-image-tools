# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Downloads an Ubuntu cloud image and converts it to raw format.
#
# Usage:
#   download-ubuntu-image.sh UBUNTU_VERSION ARCH OUTPUT_DIR
#
#     UBUNTU_VERSION: Ubuntu version (e.g., 22.04, 24.04).
#     ARCH: Architecture (e.g., amd64, arm64).
#     OUTPUT_DIR: The directory to output the raw image to.

set -eu

UBUNTU_VERSION="$1"
ARCH="$2"
OUTPUT_DIR="$3"

mkdir -p "$OUTPUT_DIR"

BASE_URL="https://cloud-images.ubuntu.com/releases"
IMG_URL="${BASE_URL}/${UBUNTU_VERSION}/release"
IMG_NAME="ubuntu-${UBUNTU_VERSION}-server-cloudimg-${ARCH}.img"

echo "Downloading Ubuntu ${UBUNTU_VERSION} (${ARCH}) cloud image..."

wget -q -O "${OUTPUT_DIR}/${IMG_NAME}" "${IMG_URL}/${IMG_NAME}"

echo "Converting qcow2 to raw..."

qemu-img convert -f qcow2 -O raw \
    "${OUTPUT_DIR}/${IMG_NAME}" \
    "${OUTPUT_DIR}/image.raw"

rm -f "${OUTPUT_DIR}/${IMG_NAME}"

echo "Ubuntu ${UBUNTU_VERSION} (${ARCH}) image ready: ${OUTPUT_DIR}/image.raw"
