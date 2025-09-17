#!/bin/bash

set -e
CONTAINER_IMAGE="$1"
DISTRO="$2"
DISTRO_VERSION="$3"
PACKAGE_LIST="$4"

# Check if the required arguments are provided
if [[ -z "$CONTAINER_IMAGE" || -z "$DISTRO" || -z "$DISTRO_VERSION" || -z "$PACKAGE_LIST" ]]; then
  echo "Usage: $0 <container_image> <distro> <DISTRO_VERSION> <package_list>"
  echo "Example: $0 mcr.microsoft.com/azurelinux/base/core:3.0 azurelinux 3.0 'pkg1 pkg2 pkg3'"
  exit 1
fi

SCRIPT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")")"
DOCKERFILE_DIR="$SCRIPT_DIR/downloader"
DOWNLOADER_RPMS_DIRS="$SCRIPT_DIR/downloadedrpms"
CONTAINER_TAG="imagecustomizertestrpms:latest"
OUT_DIR="$DOWNLOADER_RPMS_DIRS/$DISTRO/$DISTRO_VERSION"
REPO_WITH_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$DISTRO-$DISTRO_VERSION-withkey.repo"
REPO_NO_KEY_FILE="$DOWNLOADER_RPMS_DIRS/rpms-$DISTRO-$DISTRO_VERSION-nokey.repo"

# Map distro to GPG key name
declare -A GPG_KEY_MAP
GPG_KEY_MAP["azurelinux"]="MICROSOFT-RPM-GPG-KEY"
GPG_KEY_MAP["fedora"]="RPM-GPG-KEY-${DISTRO_VERSION}-fedora"

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

docker run \
  --rm \
   -v $OUT_DIR:/rpmsdir:z \
   "$CONTAINER_TAG" \
   cp -r /etc/pki/rpm-gpg/. "/rpmsdir"

# Create repo file with the appropriate GPG key, if available
GPG_KEY_NAME="${GPG_KEY_MAP[$DISTRO]:-}"

if [[ -n "$GPG_KEY_NAME" ]]; then
  KEY_PATH="$OUT_DIR/$GPG_KEY_NAME"
  if [[ -f "$KEY_PATH" ]]; then
    cat << EOF > "$REPO_WITH_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=file://$KEY_PATH
EOF
  else
    echo "GPG key '$GPG_KEY_NAME' not found in $OUT_DIR; skipping repo file with gpg key" >&2
  fi
else
  echo "No GPG key configured for distro '$DISTRO'; skipping repo file with gpg key" >&2
fi


cat << EOF > "$REPO_NO_KEY_FILE"
[localrpms]
name=Local RPMs repo
baseurl=file://$OUT_DIR
enabled=1
gpgcheck=0
repo_gpgcheck=0
EOF