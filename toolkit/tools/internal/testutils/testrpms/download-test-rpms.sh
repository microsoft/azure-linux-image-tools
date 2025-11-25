#!/bin/bash

set -e
CONTAINER_IMAGE="$1"
IMAGE_VERSION="$2"
PACKAGE_LIST="$3"

# Check if the required arguments are provided
if [[ -z "$CONTAINER_IMAGE" || -z "$IMAGE_VERSION" || -z "$PACKAGE_LIST" ]]; then
  echo "Usage: $0 <container_image> <image_version> <package_list>"
  echo "Example: $0 mcr.microsoft.com/azurelinux/base/core:3.0 azurelinux/3.0 'pkg1 pkg2 pkg3'"
  exit 1
fi

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"

# Determine which Dockerfile to use based on the container image
if [[ "$CONTAINER_IMAGE" == *"fedora"* ]]; then
  DOCKERFILE_DIR="$SCRIPT_DIR/downloader-fedora"
else
  DOCKERFILE_DIR="$SCRIPT_DIR/downloader"
fi

DOWNLOADER_RPMS_DIRS="$SCRIPT_DIR/downloadedrpms"
CONTAINER_TAG="imagecustomizertestrpms:latest"
OUT_DIR="$DOWNLOADER_RPMS_DIRS/$IMAGE_VERSION"
REPO_WITH_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-${IMAGE_VERSION//\//-}-withkey.repo"
REPO_NO_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-${IMAGE_VERSION//\//-}-nokey.repo"

mkdir -p "$OUT_DIR"

# Build a container image that contains the RPMs.
docker build \
  --build-arg "baseimage=$CONTAINER_IMAGE" \
  --build-arg "package_list=$PACKAGE_LIST" \
  --tag "$CONTAINER_TAG" \
  "$DOCKERFILE_DIR"

# Extract the RPM files.
docker run \
  --rm \
   -v $OUT_DIR:/rpmsdir:z \
   "$CONTAINER_TAG" \
   cp -r /downloadedrpms/. "/rpmsdir"

# Copy GPG keys if available (Azure Linux has them, Fedora may not)
docker run \
  --rm \
   -v $OUT_DIR:/rpmsdir:z \
   "$CONTAINER_TAG" \
   sh -c 'if [ -d /etc/pki/rpm-gpg ]; then cp -r /etc/pki/rpm-gpg/. "/rpmsdir" 2>/dev/null || true; fi'

# Create repo files.
cat << REPOEOF > "$REPO_WITH_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=file://$OUT_DIR/MICROSOFT-RPM-GPG-KEY
REPOEOF

cat << REPOEOF > "$REPO_NO_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=0
repo_gpgcheck=0
REPOEOF
