#!/bin/sh

Ask Copilot about this file-diff
echo "Running mountbootpartition.sh" 

type getarg > /dev/null 2>&1 || . /lib/dracut-lib.sh

bootPartitionUuid=$(getarg pre.verity.mount)

if [[ "$bootPartitionUuid" == "" ]]; then
    exit 0
fi

mkdir -p /boot
mount -U $bootPartitionUuid /boot

echo "done" > /run/boot-parition-mount-ready.sem
