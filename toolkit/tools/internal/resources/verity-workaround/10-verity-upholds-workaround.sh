#!/bin/bash
# Verity race condition workaround for systemd-veritysetup@root.service
#
# This dracut cmdline hook runs early in initramfs (after systemd generators
# but before device units are processed). It ensures that when partition
# device units appear, they actively start the veritysetup service via
# Upholds= directives, rather than passively satisfying BindsTo= dependencies.
#
# Without this workaround, there is a race condition where a partition device
# unit can appear and be consumed by systemd before the BindsTo= relationship
# with systemd-veritysetup@root.service is fully established. This causes the
# veritysetup service to never start, leaving the dm-verity root device (dm-0)
# unmounted and the boot hanging at sysroot.mount.
#
# This workaround is needed for both UKI and non-UKI verity images. For
# non-UKI images, trident regenerates the initrd and injects this hook at
# runtime. For UKI images, the initrd is baked into the signed UKI binary
# and cannot be regenerated, so the hook must be included at image build time.
#
# The script is safe to include unconditionally — it is a no-op when the
# veritysetup service file does not exist (non-verity boots).

SERVICE_FILE=/run/systemd/generator/systemd-veritysetup\@root.service
OVERRIDE_ROOT=/etc/systemd/system

if [ -f $SERVICE_FILE ]; then
    echo "File $SERVICE_FILE exists. Injecting verity workaround..."
    SERVICE_NAME=$(basename $SERVICE_FILE)
    echo "Service name: $SERVICE_NAME"
    PARTITIONS=$(cat $SERVICE_FILE | sed -n 's/BindsTo=//p')
    for PARTITION in $PARTITIONS; do
        echo "Injecting override for partition: $PARTITION"
        mkdir -p $OVERRIDE_ROOT/$PARTITION.d/
        OVERRIDE_FILE=$OVERRIDE_ROOT/$PARTITION.d/override.conf
        cat << EOF > $OVERRIDE_FILE
[Unit]
Upholds=$SERVICE_NAME
EOF
        echo "Created '$OVERRIDE_FILE' with contents:"
        cat $OVERRIDE_FILE
        printf "\n"
    done
    systemctl daemon-reload
fi
