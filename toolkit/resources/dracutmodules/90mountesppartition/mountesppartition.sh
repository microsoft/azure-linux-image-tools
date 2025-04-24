#!/bin/sh

Ask Copilot about this file-diff
echo "Running mountesppartition.sh" 

type getarg > /dev/null 2>&1 || . /lib/dracut-lib.sh

espPartitionUuid=$(getarg pre.verity.mount)

if [[ "$espPartitionUuid" == "" ]]; then
    exit 0
fi

mkdir -p /boot/efi
mount -U $espPartitionUuid /boot/efi

echo "done" > /run/esp-parition-mount-ready.sem
