#!/usr/bin/env bash

set -euo pipefail
shopt -s nullglob

exit_with_usage() {
    local error_msg="$1"

    echo "Usage: run.sh <VERSION> [extra imagecustomizer args]"
    echo ""
    echo "This script pulls a base image of the specified version from the Microsoft Container Registry (MCR) and runs"
    echo "imagecustomizer on it."
    echo ""
    echo "Cross-architecture builds are not available, so the architecture of the base image pulled will always match"
    echo "the architecture of the host system."
    echo ""
    echo "The base image is pulled from the MCR and stored in /container/base."
    echo ""
    echo "Arguments:"
    echo "  VERSION  The version of the image to use as the base image for the imagecustomizer run. It can be in the"
    echo "           format MAJOR.MINOR.DATE or MAJOR.MINOR.LATEST_TAG. The DATE format is YYYYMMDD. The LATEST_TAG"
    echo "           is 'latest' or any tag starting with 'latest-' (e.g. 'latest-preview') that is available in the"
    echo "           registry."
    echo ""
    echo "Options:"
    echo "  -h, --help  Show this help message and exit."
    echo ""
    echo "Environment variables:"
    echo ""
    echo "  BASE_IMAGE_NAME  The name of the base image to use (default: 'minimal-os')."
    echo "  DEVELOPER_MODE   If non-empty, the script will pull from the development registry instead of MCR."
    echo ""

    if [[ -n "$error_msg" ]]; then
        echo "Error: $error_msg" >&2
        exit 1
    fi

    exit 0
}

ARG_VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            exit_with_usage
            ;;
        -*)
            exit_with_usage "unknown option: '$1'"
            ;;
        *)
            ARG_VERSION="$1"
            shift
            break
            ;;
    esac
done

if [[ -z "$ARG_VERSION" ]]; then
    exit_with_usage "missing required argument: 'VERSION'"
fi

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)
        PLATFORM="linux/amd64"
        ;;
    aarch64|arm64)
        PLATFORM="linux/arm64"
        ;;
    *)
        echo "Error: unsupported host arch '$ARCH'"
        exit 1
        ;;
esac

OCI_ARTIFACT_REGISTRY="mcr.microsoft.com"
if [[ -n "${DEVELOPER_MODE:-}" ]]; then
    OCI_ARTIFACT_REGISTRY="acrafoimages.azurecr.io"
fi

BASE_IMAGE_NAME="${BASE_IMAGE_NAME:-seed}"
VERSION_MAJOR_MINOR="$(echo "$ARG_VERSION" | cut -d'.' -f1-2)"
OCI_ARTIFACT_REPOSITORY="azurelinux/$VERSION_MAJOR_MINOR/image/$BASE_IMAGE_NAME"

VERSION_FINAL_PARTS="$(echo "$ARG_VERSION" | cut -d'.' -f3-)"
if [[ "$VERSION_FINAL_PARTS" == latest || "$VERSION_FINAL_PARTS" == latest-* ]]; then
    OCI_ARTIFACT_TAG="$VERSION_FINAL_PARTS"
else
    OCI_ARTIFACT_TAG="$ARG_VERSION"
fi

OCI_ARTIFACT_PATH="$OCI_ARTIFACT_REGISTRY/$OCI_ARTIFACT_REPOSITORY:$OCI_ARTIFACT_TAG"
echo "Pulling OCI artifact: '$OCI_ARTIFACT_PATH' (platform='$PLATFORM')"

ARTIFACT_DIR="/container/base"
oras pull --platform "$PLATFORM" "$OCI_ARTIFACT_PATH" --output "$ARTIFACT_DIR"

# Inspect the OCI artifact manifest to dynamically detect the image file name. They are always named 'image' but may
# have any supported extension ('.vhdx', '.vhdx', etc.). File names in the OCI artifact that do not end with .spdx.json
# or .spdx.json.sig are considered image files. There should only be one, but if there are multiple, the first one is
# used and a warning is printed.
OCI_ARTIFACT_FILE_NAMES=($(oras manifest fetch --platform "$PLATFORM" "$OCI_ARTIFACT_PATH" | \
    jq -r '.layers[].annotations["org.opencontainers.image.title"]'))
IMAGE_FILE_NAME=""
for name in "${OCI_ARTIFACT_FILE_NAMES[@]}"; do
    if [[ "$name" == *.spdx.json || "$name" == *.spdx.json.sig ]]; then
        continue
    fi

    if [[ -n "$IMAGE_FILE_NAME" ]]; then
        echo "Warning: multiple images downloaded, using the first: '$IMAGE_FILE_NAME'" >&2
        continue
    fi

    IMAGE_FILE_NAME="$name"
done

if [[ -z "$IMAGE_FILE_NAME" ]]; then
    echo "Error: no image file found in the OCI artifact '$OCI_ARTIFACT_PATH'" >&2
    exit 1
fi

imagecustomizer --image-file "$ARTIFACT_DIR/$IMAGE_FILE_NAME" "$@"
