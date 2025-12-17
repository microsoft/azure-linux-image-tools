#!/bin/sh

echo "mountbootpartition-generator.sh: Running" > /dev/kmsg

command -v getarg > /dev/null || . /lib/dracut-lib.sh

# systemd provides a directory to place the generated unit files.
outputDir="$1"

bootPartitionUuid=$(getarg pre.verity.mount) || bootPartitionUuid=""
if [[ -z "$bootPartitionUuid" ]]; then
    echo "mountbootpartition-generator.sh: cmdline arg pre.verity.mount is not specified" > /dev/kmsg
    exit 0
fi

echo "mountbootpartition-generator.sh: Adding boot.mount" > /dev/kmsg

bootMountMonitorUnitFile="$outputDir/boot.mount"

cat <<EOF > $bootMountMonitorUnitFile
[Unit]
Description=/boot
RequiresMountsFor=/dev/disk/by-uuid/$bootPartitionUuid
After=dev-disk-by\\x2duuid-${bootPartitionUuid//-/\\x2d}.device
Before=veritysetup-pre.target
Wants=veritysetup-pre.target

[Mount]
What=UUID=$bootPartitionUuid
Where=/boot
Options=ro
EOF

echo "mountbootpartition-generator.sh: Enabling boot.mount" > /dev/kmsg

enableDir="$outputDir/local-fs.target.wants"
mkdir -p "$enableDir"

ln -s "$bootMountMonitorUnitFile" "$enableDir"

echo "mountbootpartition-generator.sh: Added /boot mount" > /dev/kmsg
