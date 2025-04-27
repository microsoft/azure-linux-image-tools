#!/bin/sh

set -x
set -e

echo "Running mountesppartition-generator.sh" > /dev/kmsg

type getarg > /dev/null 2>&1 || . /lib/dracut-lib.sh

# Read ESP partition UUID from kernel command line
espPartitionUuid=$(getarg pre.verity.mount)

if [ -z "$espPartitionUuid" ]; then
    echo "No pre.verity.mount= UUID provided, skipping ESP mount generation." > /dev/kmsg
    exit 0
fi

# Prepare mount point and mount unit path
mountPoint="/boot/efi"
mountUnitFile="/etc/systemd/system/boot-efi.mount"

mkdir -p "$(dirname "$mountUnitFile")"

# Write the .mount unit
cat <<EOF > "$mountUnitFile"
[Unit]
Description=Mount ESP Partition to $mountPoint
DefaultDependencies=no
Before=veritysetup-pre.target
Wants=veritysetup-pre.target

[Mount]
What=UUID=$espPartitionUuid
Where=$mountPoint
Type=vfat
Options=defaults

[Install]
WantedBy=local-fs.target
WantedBy=veritysetup-pre.target
EOF

chmod 644 "$mountUnitFile"

echo "Successfully generated $mountUnitFile" > /dev/kmsg
