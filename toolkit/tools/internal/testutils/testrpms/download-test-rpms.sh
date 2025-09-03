#!/bin/bash

set -e
CONTAINER_IMAGE="$1"
DISTRO="$2"
IMAGE_VERSION="$3"
IMAGE_CREATOR="$4"

# Check if the required arguments are provided
if [[ -z "$CONTAINER_IMAGE" || -z "$DISTRO" || -z "$IMAGE_VERSION" || -z "$IMAGE_CREATOR" ]]; then
  echo "Usage: $0 <container_image> <distro> <image_version> <image_creator>"
  echo "Example: $0 mcr.microsoft.com/azurelinux/base/core:3.0 azurelinux 3.0 true"
  exit 1
fi

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"
DOCKERFILE_DIR="$SCRIPT_DIR/downloader"
DOWNLOADER_RPMS_DIRS="$SCRIPT_DIR/downloadedrpms"
CONTAINER_TAG="imagecustomizertestrpms:latest"
OUT_DIR="$DOWNLOADER_RPMS_DIRS/$DISTRO/$IMAGE_VERSION"
REPO_WITH_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$DISTRO-$IMAGE_VERSION-withkey.repo"
REPO_NO_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$DISTRO-$IMAGE_VERSION-nokey.repo"

# Declarative configuration maps
TESTDATA_DIR="$SCRIPT_DIR/../../../../tools/pkg/imagecreatorlib/testdata"

# Map distro to config file
declare -A DISTRO_CONFIG_MAP
DISTRO_CONFIG_MAP["azurelinux"]="minimal-os.yaml"
DISTRO_CONFIG_MAP["fedora"]="fedora.yaml"

# Use a single unified Dockerfile for all distros
DOCKERFILE_NAME="Dockerfile"

mkdir -p "$OUT_DIR"

# Get configuration file for the distro
CONFIG_FILE="${DISTRO_CONFIG_MAP[$DISTRO]}"

# Validate that we have configuration for this distro
if [[ -z "$CONFIG_FILE" ]]; then
  echo "Error: Unsupported distro '$DISTRO'"
  echo "Supported distros: ${!DISTRO_CONFIG_MAP[@]}"
  exit 1
fi

echo "Using config file: $CONFIG_FILE"
echo "Using unified Dockerfile: $DOCKERFILE_NAME"

# Extract package list from config file
PACKAGE_LIST=""
if command -v python3 >/dev/null 2>&1; then
  CONFIG_PATH="$TESTDATA_DIR/$CONFIG_FILE"
  if [[ -f "$CONFIG_PATH" ]]; then
    PACKAGE_LIST=$(python3 "$SCRIPT_DIR/extract_packages.py" "$CONFIG_PATH")
  else
    echo "Warning: Config file '$CONFIG_PATH' not found"
  fi
else
  echo "Warning: python3 not found, skipping package extraction from config file"
fi

echo "Package list: $PACKAGE_LIST"

# Add common packages needed for testing
COMMON_PACKAGES="jq golang"

# Combine package lists
if [[ -n "$PACKAGE_LIST" ]]; then
    FINAL_PACKAGE_LIST="$COMMON_PACKAGES $PACKAGE_LIST"
else
    FINAL_PACKAGE_LIST="$COMMON_PACKAGES"
fi

echo "Final package list including common packages: $FINAL_PACKAGE_LIST"

# Build a container image that contains the RPMs.
docker build \
  --build-arg "baseimage=$CONTAINER_IMAGE" \
  --build-arg "package_list=$FINAL_PACKAGE_LIST" \
  --tag "$CONTAINER_TAG" \
  --file "$DOCKERFILE_DIR/$DOCKERFILE_NAME" \
  "$DOCKERFILE_DIR"

# Extract the RPM files.
docker run \
  --rm \
   -v $OUT_DIR:/rpmsdir:z \
   "$CONTAINER_TAG" \
   cp -r /downloadedrpms/. "/rpmsdir"

docker run \
  --rm \
   -v $OUT_DIR:/rpmsdir:z \
   "$CONTAINER_TAG" \
   cp -r /etc/pki/rpm-gpg/. "/rpmsdir"

# Create repo files.
cat << EOF > "$REPO_WITH_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=file://$OUT_DIR/MICROSOFT-RPM-GPG-KEY
EOF

cat << EOF > "$REPO_NO_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=0
repo_gpgcheck=0
EOF
