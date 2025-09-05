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

# Declarative configuration
TESTDATA_DIR="$SCRIPT_DIR/../../../pkg/imagecreatorlib/testdata"
DISTRO="azurelinux"
CONFIG_FILE="minimal-os.yaml"

# Initialize package list
PACKAGE_LIST=""

# Handle tools file creation and package extraction for imagecreator testing
if [ "$IMAGE_CREATOR" = "true" ]; then
  echo "Creating tools file: $TOOLS_FILE"
  $SCRIPT_DIR/create-tools-file.sh "$CONTAINER_IMAGE" "$TOOLS_FILE"
  echo "Tools file created successfully."

  # Check for python3 availability
  if ! command -v python3 >/dev/null 2>&1; then
    echo "Error: python3 is required but not found in PATH"
    echo "Please install python3 to extract package lists from config files"
    exit 1
  fi

  # Extract package list from config file
  CONFIG_PATH="$TESTDATA_DIR/$CONFIG_FILE"
  if [[ ! -f "$CONFIG_PATH" ]]; then
    echo "Error: Config file '$CONFIG_PATH' not found"
    echo "Expected config file at: $CONFIG_PATH"
    exit 1
  fi
  
  PACKAGE_LIST=$(python3 "$SCRIPT_DIR/extract_packages.py" "$CONFIG_PATH")
  echo "Package list from $CONFIG_FILE: $PACKAGE_LIST"
else
  echo "Skipping tools file creation and package extraction."
fi

# Combine with common testing packages
FINAL_PACKAGE_LIST="jq golang $PACKAGE_LIST"

echo "Final package list: $FINAL_PACKAGE_LIST"

# Download the RPMs
echo "Downloading test RPMs for $DISTRO version: $IMAGE_VERSION"
$SCRIPT_DIR/download-test-rpms.sh "$CONTAINER_IMAGE" "$IMAGE_VERSION" "$FINAL_PACKAGE_LIST"
echo "Test RPMs downloaded successfully."
