#!/bin/bash

set -e
CONTAINER_IMAGE="$1"
IMAGE_VERSION="$2"
IMAGE_CREATOR="$3"

# Check if the required arguments are provided
if [[ -z "$CONTAINER_IMAGE" || -z "$IMAGE_VERSION" || -z "$IMAGE_CREATOR" ]]; then
  echo "Usage: $0 <container_image> <image_version> <image_creator>"
  echo "Example: $0 mcr.microsoft.com/azurelinux/base/core:3.0 3.0 true"
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

# Determine which Dockerfile to use and extract package list based on the base image
TESTDATA_DIR="$SCRIPT_DIR/../../../../tools/pkg/imagecreatorlib/testdata"
DOCKERFILE_NAME="Dockerfile"
PACKAGE_LIST=""

if [[ "$CONTAINER_IMAGE" == *"fedora"* ]]; then
  DOCKERFILE_NAME="Dockerfile.fedora"
  echo "Using Fedora-specific Dockerfile"
  # Extract packages from fedora.yaml
  if command -v python3 >/dev/null 2>&1; then
    PACKAGE_LIST=$(python3 -c "
import yaml
import sys
try:
    with open('$TESTDATA_DIR/fedora.yaml', 'r') as f:
        data = yaml.safe_load(f)
        packages = data.get('os', {}).get('packages', {}).get('install', [])
        print(' '.join(packages))
except Exception as e:
    print('', file=sys.stderr)
")
  fi
elif [[ "$CONTAINER_IMAGE" == *"azurelinux"* ]] || [[ "$CONTAINER_IMAGE" == *"mariner"* ]]; then
  DOCKERFILE_NAME="Dockerfile.azurelinux"
  echo "Using Azure Linux-specific Dockerfile"
  # Extract packages from minimal-os.yaml
  if command -v python3 >/dev/null 2>&1; then
    PACKAGE_LIST=$(python3 -c "
import yaml
import sys
try:
    with open('$TESTDATA_DIR/minimal-os.yaml', 'r') as f:
        data = yaml.safe_load(f)
        packages = data.get('os', {}).get('packages', {}).get('install', [])
        print(' '.join(packages))
except Exception as e:
    print('', file=sys.stderr)
")
  fi
else
  echo "Warning: Could not determine distribution from image name '$CONTAINER_IMAGE', using default Dockerfile"
fi

echo "Package list: $PACKAGE_LIST"

# Build a container image that contains the RPMs.
docker build \
  --build-arg "baseimage=$CONTAINER_IMAGE" \
  --build-arg "imagecreator=$IMAGE_CREATOR" \
  --build-arg "package_list=$PACKAGE_LIST" \
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
