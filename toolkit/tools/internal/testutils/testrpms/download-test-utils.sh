#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"

AZURELINUX_2_CONTAINER_IMAGE="mcr.microsoft.com/cbl-mariner/base/core:2.0"
AZURELINUX_3_CONTAINER_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"
FEDORA_42_CONTAINER_IMAGE="quay.io/fedora/fedora:42"

DISTRO="azurelinux"
DISTRO_VERSION="3.0"

IMAGE_CREATOR="false"
CONTAINER_REGISTRY=""

while getopts "d:t:s:r:" flag
do
    case "${flag}" in
        d) DISTRO="$OPTARG";;
        t) DISTRO_VERSION="$OPTARG";;
        s) IMAGE_CREATOR="$OPTARG";;
        r) CONTAINER_REGISTRY="$OPTARG";;
        h) ;;&
        ?) echo "Usage: download-test-utils.sh [-d DISTRO] [-t DISTRO_VERSION] [-s IMAGE_CREATOR] [-r CONTAINER_REGISTRY]"
            echo ""
            echo "Args:"
            echo "  -d DISTRO              The distribution to use (azurelinux or fedora). Default: azurelinux"
            echo "  -t DISTRO_VERSION      The image version to download the RPMs for (2.0, 3.0 for Azure Linux or 42 for Fedora)."
            echo "  -s IMAGE_CREATOR       If set to true, the script will create a tar.gz file with the tools and download the rpms needed to test imagecreator."
            echo "  -r CONTAINER_REGISTRY  Container registry URL to use for Fedora images (e.g., myacr.azurecr.io)."
            echo "  -h Show help"
            exit 1;;
    esac
done

# Override Fedora container image if a container registry is provided
if [[ -n "$CONTAINER_REGISTRY" ]]; then
    FEDORA_42_CONTAINER_IMAGE="${CONTAINER_REGISTRY}/fedora/fedora:42"
fi

# Determine the tools file name based on the distro and image version
BUILD_DIR="$SCRIPT_DIR/build"
mkdir -p "$BUILD_DIR"
TOOLS_FILE="$BUILD_DIR/tools-$DISTRO-$DISTRO_VERSION.tar.gz"

# Determine config file based on distro
TESTDATA_DIR="$SCRIPT_DIR/../../../pkg/imagecreatorlib/testdata"

case "${DISTRO}" in
  azurelinux)
    case "${DISTRO_VERSION}" in
      3.0)
        CONTAINER_IMAGE="$AZURELINUX_3_CONTAINER_IMAGE"
        ;;
      2.0)
        CONTAINER_IMAGE="$AZURELINUX_2_CONTAINER_IMAGE"
        ;;
      *)
        echo "error: unsupported Azure Linux version: $DISTRO_VERSION"
        exit 1;;
    esac
    CONFIG_FILE="minimal-os.yaml"
    ;;
  fedora)
    case "${DISTRO_VERSION}" in
      42)
        CONTAINER_IMAGE="$FEDORA_42_CONTAINER_IMAGE"
        ;;
      *)
        echo "error: unsupported Fedora version: $DISTRO_VERSION"
        exit 1;;
    esac
    CONFIG_FILE="fedora.yaml"
    ;;
  *)
    echo "error: unsupported distro: $DISTRO"
    exit 1;;
esac

set -x

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
# Add dnf for Fedora, but not for Azure Linux (uses tdnf)
if [ "$DISTRO" = "fedora" ]; then
  FINAL_PACKAGE_LIST="jq golang $PACKAGE_LIST"
else
  FINAL_PACKAGE_LIST="jq golang $PACKAGE_LIST"
fi

echo "Final package list: $FINAL_PACKAGE_LIST"

# Create output directory structure for distro
DOWNLOADER_RPMS_DIR="$SCRIPT_DIR/downloadedrpms/$DISTRO"
mkdir -p "$DOWNLOADER_RPMS_DIR"

# Download the RPMs
echo "Downloading test RPMs for $DISTRO version: $DISTRO_VERSION"
$SCRIPT_DIR/download-test-rpms.sh "$CONTAINER_IMAGE" "$DISTRO/$DISTRO_VERSION" "$FINAL_PACKAGE_LIST"
echo "Test RPMs downloaded successfully."
