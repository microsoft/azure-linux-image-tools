#!/bin/bash

set -e
CONTAINER_IMAGE="$1"
IMAGE_CREATOR="$2"


# Check if the required arguments are provided
if [[ -z "$CONTAINER_IMAGE" || -z "$IMAGE_CREATOR" ]]; then
  echo "Usage: $0 <container_image> <image_creator>"
  echo "Example: $0 mcr.microsoft.com/azurelinux/base/core:3.0 true"
  exit 1
fi

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"
DOCKERFILE_DIR="$SCRIPT_DIR/downloader"
DOWNLOADER_RPMS_DIRS="$SCRIPT_DIR/downloadedrpms"
CONTAINER_TAG="imagecustomizertestrpms:latest"
OUT_DIR="$DOWNLOADER_RPMS_DIRS/$IMAGE_VERSION"
REPO_WITH_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$IMAGE_VERSION-withkey.repo"
REPO_NO_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$IMAGE_VERSION-nokey.repo"

mkdir -p "$OUT_DIR"

# Build a container image that contains the RPMs.
docker build \
  --build-arg "baseimage=$CONTAINER_IMAGE" \
  --build-arg "imagecreator=$IMAGE_CREATOR" \
  --tag "$CONTAINER_TAG" \
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
