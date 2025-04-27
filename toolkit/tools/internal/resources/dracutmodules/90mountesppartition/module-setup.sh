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
    # If systemd-initrd is used, install the generator
    if dracut_module_included "systemd-initrd"; then
        inst_script "$moddir/mountesppartition-generator.sh" "$systemdutildir/system-generators/dracut-mountesppartition-generator"
    fi
}
