#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"

AZURELINUX_2_CONTAINER_IMAGE="mcr.microsoft.com/cbl-mariner/base/core:2.0"
AZURELINUX_3_CONTAINER_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"
FEDORA_42_CONTAINER_IMAGE="registry.fedoraproject.org/fedora:42"

DISTRO="azurelinux"
IMAGE_VERSION="2.0"

IMAGE_CREATOR="false"

while getopts "d:t:s:" flag
do
    case "${flag}" in
        d) DISTRO="$OPTARG";;
        t) IMAGE_VERSION="$OPTARG";;
        s) IMAGE_CREATOR="$OPTARG";;
        h) ;;&
        ?) echo "Usage: download-test-utils.sh [-d DISTRO] [-t IMAGE_VERSION] [-s IMAGE_CREATOR]"
            echo ""
            echo "Args:"
            echo "  -d DISTRO          The distribution to use (azurelinux or fedora). Default: azurelinux"
            echo "  -t IMAGE_VERSION   The image version to download the RPMs for (2.0, 3.0 for Azure Linux or 42 for Fedora)."
            echo "  -s IMAGE_CREATOR   If set to true, the script will create a tar.gz file with the tools and download the rpms needed to test imagecreator."
            echo "  -h Show help"
            exit 1;;
    esac
done

# Determine the tools file name based on the distro and image version
BUILD_DIR="$SCRIPT_DIR/build"
mkdir -p "$BUILD_DIR"

# Create consistent naming for tools tarball: tools-{distro}-{version}.tar.gz
TOOLS_FILE="$BUILD_DIR/tools-${DISTRO}-${IMAGE_VERSION}.tar.gz"


case "${DISTRO}-${IMAGE_VERSION}" in
  azurelinux-3.0)
    CONTAINER_IMAGE="$AZURELINUX_3_CONTAINER_IMAGE"
    ;;
  azurelinux-2.0)
    CONTAINER_IMAGE="$AZURELINUX_2_CONTAINER_IMAGE"
    ;;
  fedora-42)
    CONTAINER_IMAGE="$FEDORA_42_CONTAINER_IMAGE"
    ;;
  *)
    echo "error: unsupported distro-version combination: $DISTRO-$IMAGE_VERSION"
    echo "Supported combinations:"
    echo "  azurelinux-2.0, azurelinux-3.0, fedora-42"
    exit 1;;
esac

set -x

# call the script to create the tools file if requested
if [ "$IMAGE_CREATOR" = "true" ]; then
  echo "Creating tools file: $TOOLS_FILE"
  $SCRIPT_DIR/create-tools-file.sh "$CONTAINER_IMAGE" "$TOOLS_FILE"
  echo "Tools file created successfully."
else
  echo "Skipping tools file creation."
fi

# call the script to download the rpms
echo "Downloading test rpms for distro: $DISTRO, version: $IMAGE_VERSION"
$SCRIPT_DIR/download-test-rpms.sh "$CONTAINER_IMAGE" "$DISTRO" "$IMAGE_VERSION" "$IMAGE_CREATOR"
echo "Test rpms downloaded successfully."
