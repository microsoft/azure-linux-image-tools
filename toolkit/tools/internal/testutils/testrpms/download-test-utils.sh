#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"

AZURELINUX_2_CONTAINER_IMAGE="mcr.microsoft.com/cbl-mariner/base/core:2.0"
AZURELINUX_3_CONTAINER_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"

IMAGE_VERSION="2.0"

IMAGE_CREATOR="false"

mkdir -p $SCRIPT_DIR/build
TOOLS_FILE="$SCRIPT_DIR/build/tools.tar.gz"

while getopts "s:t:" flag
do
    case "${flag}" in
        s) IMAGE_CREATOR="$OPTARG";;
        t) IMAGE_VERSION="$OPTARG";;
        h) ;;&
        ?)
            echo "Usage: download-test-utils.sh [-t IMAGE_VERSION] [-s IMAGE_CREATOR]"
            echo ""
            echo "Args:"
            echo "  -t IMAGE_VERSION   The Azure Image version to download the RPMs for."
            echo "  -s IMAGE_CREATOR   If set to true, the script will create a tar.gz file with the tools and download the rpms needed to test imagecreator."
            echo "  -h Show help"
            exit 1;;
    esac
done


case "${IMAGE_VERSION}" in
  3.0)
    CONTAINER_IMAGE="$AZURELINUX_3_CONTAINER_IMAGE"
    ;;
  2.0)
    CONTAINER_IMAGE="$AZURELINUX_2_CONTAINER_IMAGE"
    ;;  
  *)
    echo "error: unsupported Azure Linux version: $IMAGE_VERSION"
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
echo "Downloading test rpms for Azure Linux version: $IMAGE_VERSION"
$SCRIPT_DIR/download-test-rpms.sh "$CONTAINER_IMAGE"  "$IMAGE_VERSION" "$IMAGE_CREATOR" 
echo "Test rpms downloaded successfully."
