#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"

AZURELINUX_2_CONTAINER_IMAGE="mcr.microsoft.com/cbl-mariner/base/core:2.0"
AZURELINUX_3_CONTAINER_IMAGE="mcr.microsoft.com/azurelinux/base/core:3.0"

# Use ACR image in CI, public registry for local development
if [ "${CI:-}" = "true" ]; then
    FEDORA_42_CONTAINER_IMAGE="martimusexternal.azurecr.io/fedora/fedora:42"
else
    FEDORA_42_CONTAINER_IMAGE="registry.fedoraproject.org/fedora:42"
fi

DISTRO="azurelinux"
DISTRO_VERSION="3.0"

IMAGE_CREATOR="false"

while getopts "d:t:s:" flag
do
    case "${flag}" in
        d) DISTRO="$OPTARG";;
        t) DISTRO_VERSION="$OPTARG";;
        s) IMAGE_CREATOR="$OPTARG";;
        h) ;;&
        ?) echo "Usage: download-test-utils.sh [-d DISTRO] [-t DISTRO_VERSION] [-s IMAGE_CREATOR]"
            echo ""
            echo "Args:"
            echo "  -d DISTRO          The distribution to use (azurelinux or fedora). Default: azurelinux"
            echo "  -t DISTRO_VERSION   The image version to download the RPMs for (2.0, 3.0 for Azure Linux or 42 for Fedora)."
            echo "  -s IMAGE_CREATOR   If set to true, the script will create a tar.gz file with the tools and download the rpms needed to test imagecreator."
            echo "  -h Show help"
            exit 1;;
    esac
done

# Determine the tools file name based on the distro and image version
BUILD_DIR="$SCRIPT_DIR/build"
mkdir -p "$BUILD_DIR"

# Create consistent naming for tools tarball: tools-{distro}-{version}.tar.gz
TOOLS_FILE="$BUILD_DIR/tools-${DISTRO}-${DISTRO_VERSION}.tar.gz"


case "${DISTRO}-${DISTRO_VERSION}" in
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
    echo "error: unsupported distro-version combination: $DISTRO-$DISTRO_VERSION"
    echo "Supported combinations:"
    echo "  azurelinux-2.0, azurelinux-3.0, fedora-42"
    exit 1;;
esac

set -x

# Initialize package list
PACKAGE_LIST=""
# Declarative configuration maps
TESTDATA_DIR="$SCRIPT_DIR/../../../../tools/pkg/imagecreatorlib/testdata"

# Map distro to config file
declare -A DISTRO_CONFIG_MAP
DISTRO_CONFIG_MAP["azurelinux"]="minimal-os.yaml"
DISTRO_CONFIG_MAP["fedora"]="fedora.yaml"

# Get configuration file for the distro
CONFIG_FILE="${DISTRO_CONFIG_MAP[$DISTRO]}"
# Validate that we have configuration for this distro
if [[ -z "$CONFIG_FILE" ]]; then
  echo "Error: Unsupported distro '$DISTRO'"
  echo "Supported distros: ${!DISTRO_CONFIG_MAP[@]}"
  exit 1
fi


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

echo "Downloading test RPMs for $DISTRO version: $DISTRO_VERSION"
$SCRIPT_DIR/download-test-rpms.sh "$CONTAINER_IMAGE" "$DISTRO" "$DISTRO_VERSION" "$FINAL_PACKAGE_LIST"
echo "Test RPMs downloaded successfully."