#!/bin/bash

check() {
    return 255
}

depends() {
    return 0
}

installkernel() {
    return 0
}

install() {
    # install utilities
    inst_multiple lsblk umount
    # generate udev rule - i.e. schedule things post udev settlement
    inst_hook pre-udev 30 "$moddir/mountesppartition-genrules.sh"
    # script to run post udev to mout
    inst_script "$moddir/mountesppartition.sh" "/sbin/mountesppartition"
    # script runs early on when systemd is initialized...
    if dracut_module_included "systemd-initrd"; then
        inst_script "$moddir/mountesppartition-generator.sh" "$systemdutildir"/system-generators/dracut-mountesppartition-generator
    fi
    dracut_need_initqueue
}
