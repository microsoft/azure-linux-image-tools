#!/bin/bash

# called by dracut
check() {
    return 255
}

# called by dracut
depends() {
    echo "systemd-initrd"
    return 0
}

# called by dracut
installkernel() {
    return 0
}

# called by dracut
install() {
    inst_script "$moddir/mountbootpartition-generator.sh" "$systemdutildir"/system-generators/dracut-mountbootpartition-generator
}
