#!/bin/bash

set -e

CONTAINER_IMAGE="$1"
TOOLS_FILE="$2"

if [[ -z "$CONTAINER_IMAGE" || -z "$TOOLS_FILE" ]]; then
  echo "Usage: $0 <container_image> <tools_file>"
  exit 1
fi

docker create --name temp-container "$CONTAINER_IMAGE"
docker export temp-container | gzip > "$TOOLS_FILE"
docker rm temp-container
