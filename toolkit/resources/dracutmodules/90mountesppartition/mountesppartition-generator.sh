#!/bin/sh

set -x
set -e

echo "Running mountesppartition-generator.sh" > /dev/kmsg

# type getarg > /dev/null 2>&1 || . /lib/dracut-lib.sh

function updateVeritySetupUnit () {
    systemdDropInDir=/etc/systemd/system
    verityDropInDir=$systemdDropInDir/systemd-veritysetup@root.service.d

    mkdir -p $verityDropInDir
    verityConfiguration=$verityDropInDir/verity-azl-extension.conf

    cat <<EOF > $verityConfiguration
[Unit]
After=espmountmonitor.service
Requires=espmountmonitor.service
EOF

    chmod 644 $verityConfiguration
    chown root:root $verityConfiguration
}

# -----------------------------------------------------------------------------
function createEspPartitionMonitorScript () {
    local espPartitionMonitorCmd=$1
    local semaphorefile=$2

    cat <<EOF > $espPartitionMonitorCmd
#!/bin/sh
while [ ! -e "$semaphorefile" ]; do
    echo "Waiting for $semaphorefile to exist..."
    sleep 1
done    
EOF
    chmod +x $espPartitionMonitorCmd
}

# -----------------------------------------------------------------------------
function createEspPartitionMonitorUnit() {
    local espPartitionMonitorCmd=$1

    espMountMonitorName="espmountmonitor.service"
    systemdDropInDir=/etc/systemd/system
    espMountMonitorDir=$systemdDropInDir
    espMountMonitorUnitFile=$espMountMonitorDir/$espMountMonitorName

    cat <<EOF > $espMountMonitorUnitFile
[Unit]
Description=esppartitionmounter
DefaultDependencies=no
[Service]
Type=oneshot
ExecStart=$espPartitionMonitorCmd
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
EOF
}

# -----------------------------------------------------------------------------

updateVeritySetupUnit

systemdScriptsDir=/usr/local/bin
espPartitionMonitorCmd=$systemdScriptsDir/esp-partition-monitor.sh
semaphorefile=/run/esp-parition-mount-ready.sem

mkdir -p $systemdScriptsDir

createEspPartitionMonitorScript $espPartitionMonitorCmd $semaphorefile
createEspPartitionMonitorUnit $espPartitionMonitorCmd

echo "mountesppartition-generator.sh completed successfully." > /dev/kmsg
