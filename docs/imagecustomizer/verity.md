# Verity Image Recommendations

The Verity-enabled root filesystem is always mounted as read-only. Its root hash
and hash tree are computed at build time and verified by systemd during the
initramfs phase on each boot. When enabling the Verity feature, it is
recommended to create a writable persistent partition for any directories that
require write access. Critical files and directories can be redirected to the
writable partition using symlinks or similar methods.

Please also note that some services and programs on Azure Linux may require
specific handling when using Verity. Depending on user needs, there are
different configuration options that offer tradeoffs between convenience and
security. Some configurations can be made flexible to allow changes, while
others may be set as immutable for enhanced security.

## Writable `/var` Partition

Many services  (e.g., auditd, docker, logrotate, etc.) require write access to
the /var directory.

### Solution: Create a Writable Persistent /var Partition

To provide the required write access, create a separate writable partition for
/var. Here is an example of how to define the partitions and filesystems in your
configuration:

```yaml
storage:
  disks:
  - partitionTableType: gpt
    maxSize: 5120M
    partitions:
    - id: boot
      start: 1M
      end: 1024M
    - id: root
      start: 1024M
      end: 3072M
    - id: roothash
      start: 3072M
      end: 3200M
    - id: var
      start: 3200M
  filesystems:
  - deviceId: boot
    type: ext4
    mountPoint:
      path: /boot
  - deviceId: root
    type: ext4
    mountPoint:
      path: /
  - deviceId: var
    type: ext4
    mountPoint:
      path: /var
```

## Network Configuration for Verity Images

In non-verity images, usually user can leverage cloud-init to provide default
networking settings. However, cloud-init fails to provision the network in
verity images since /etc is not writable.

### Solution: Specify Network Settings Manually

For verity images, it's recommended to specify network settings manually. Here
is an example network configuration that can be added to the `additionalFiles`
in your configuration YAML file:

```yaml
os:
  additionalFiles:
  - content: |
      # SPDX-License-Identifier: MIT-0
      #
      # This example config file is installed as part of systemd.
      # It may be freely copied and edited (following the MIT No Attribution license).
      #
      # To use the file, one of the following methods may be used:
      # 1. add a symlink from /etc/systemd/network to the current location of this file,
      # 2. copy the file into /etc/systemd/network or one of the other paths checked
      #    by systemd-networkd and edit it there.
      # This file should not be edited in place, because it'll be overwritten on upgrades.

      # Enable DHCPv4 and DHCPv6 on all physical ethernet links
      [Match]
      Kind=!*
      Type=ether

      [Network]
      DHCP=yes
    destination: /etc/systemd/network/89-ethernet.network
    permissions: "664"
```

## cloud-init

cloud-init has various features to configure the system (e.g., user accounts,
networking, etc.), but many of these require the /etc directory to be writable.
In verity-protected images with a read-only root filesystem, cloud-init cannot
perform these configurations effectively.

### Solution: Disable cloud-init

Given the limitations, the general recommendation is to disable cloud-init in
verity images to prevent potential issues.

```yaml
os:
  services:
    disable:
    - cloud-init
```

## sshd

The `sshd` service requires write access to the SSH host keys, which by default
are stored in `/etc/ssh`. However, with the root filesystem being read-only,
this prevents `sshd` from running correctly.

### Solution: Create a writable persistent partition and redirect SSH host keys

To resolve this, create a writable partition for `/var` and redirect the SSH
host keys from `/etc` to `/var`. This ensures that `sshd` can write and access
the necessary keys without encountering issues due to the read-only root
filesystem.

Example Image Config:

```yaml
storage:
  disks:
  - partitionTableType: gpt
    maxSize: 5120M
    partitions:
    - id: boot
      start: 1M
      end: 1024M
    - id: root
      start: 1024M
      end: 3072M
    - id: roothash
      start: 3072M
      end: 3200M
    - id: var
      start: 3200M
  verity:
  - id: verityroot
    name: root
    dataDeviceId: root
    hashDeviceId: roothash
    corruptionOption: panic
  filesystems:
  - deviceId: boot
    type: ext4
    mountPoint:
      path: /boot
  - deviceId: verityroot
    type: ext4
    mountPoint:
      path: /
  - deviceId: var
    type: ext4
    mountPoint:
      path: /var
os:
  additionalFiles:
    # Change the directory that the sshd-keygen service writes the SSH host keys to.
  - content: |
      [Unit]
      Description=Generate sshd host keys
      ConditionPathExists=|!/var/etc/ssh/ssh_host_rsa_key
      ConditionPathExists=|!/var/etc/ssh/ssh_host_ecdsa_key
      ConditionPathExists=|!/var/etc/ssh/ssh_host_ed25519_key
      Before=sshd.service

      [Service]
      Type=oneshot
      RemainAfterExit=yes
      ExecStart=/usr/bin/ssh-keygen -A -f /var

      [Install]
      WantedBy=multi-user.target
    destination: /usr/lib/systemd/system/sshd-keygen.service
    permissions: "664"
  services:
    enable:
    - sshd
scripts:
  postCustomization:
    # Move the SSH host keys off of the read-only /etc directory, so that sshd can run.
  - content: |
      # Move the SSH host keys off the read-only /etc directory, so that sshd can run.
      SSH_VAR_DIR="/var/etc/ssh/"
      mkdir -p "$SSH_VAR_DIR"

      cat << EOF >> /etc/ssh/sshd_config

      HostKey $SSH_VAR_DIR/ssh_host_rsa_key
      HostKey $SSH_VAR_DIR/ssh_host_ecdsa_key
      HostKey $SSH_VAR_DIR/ssh_host_ed25519_key
      EOF
  name: ssh-move-host-keys.sh
```

## systemd-growfs-root

This service attempts to resize the root filesystem, which fails since verity
makes the root filesystem readonly and a fixed size.

### Solution 1: Do nothing

Since the root filesystem is readonly, the `systemd-growfs-root` service will
fail. However, the only impact will be an error in the boot logs.

### Solution 2: Disable service

Disabling the service removes the error from the boot logs.

```yaml
os:
  services:
    disable:
    - systemd-growfs-root
```

### Signing Verity Hashes

To sign verity hashes, we need to:

- Invoke the Azure Linux Image Customizer to:
  - Configure dm-verity to check for signed hashes.
  - Calculate the hashes and export them.
- Sign the exported hashes.
- Invoke the Azure Linux Image Customizer to:
  - Re-inject the exported hashes into the image.

The following commadline switches are used to achieve that:

- First Invocation Switches:
  - `--output-verity-hashes`
    - Exports the dm-verity calculated root hashes.
      - Each exported hash will be stored in a separate text file where its
        name is the verity device concatenated with 'hash'.
      - The exported hash files will be placed in the folder specified by the
        value of `--output-verity-hashes-dir`
  - `--output-verity-hashes-dir`
    - Specifies where to saved the exported hashes.
  - `--require-signed-rootfs-root-hash`
    - When specified, the rootfs signed hash is expected to be at
      `/boot/<verity-device-name>.hash.sig`. If absent or not signed
      properly, the rootfs verity device verification will fail.
  - `--require-signed-root-hashes`
    - When specified, all verity devices will be required to have signed root
      hashes (rootfs, containers, etc).

For testing, the user may choose to export the hashes without requiring
signatures.

- Second Invocation Switches:
  - `--input-signed-verity-hashes-files <file0> [<file1>..<filen>]`
    - The list of files to import and place on the boot partition at `/boot`.
    - Each file name must be on the form `<verity-device-name>.hash.sig`.


```bash
imageCustomizerPath="./imagecustomizer"
inputConfigFile="./verity-test.yaml"
inputImage="./core-3.0.20241216.vhdx"
buildDir="./build"

outputFormat="qcow2"
outputBaseName="verity-$(date +'%Y%m%d-%H%M').$outputFormat"
outputDir="./output"
verityImage="$outputDir/$outputBaseName"

hashFilesDir="./temp/root-hashes"
hashFile="$hashFilesDir/root.hash"

rm -rf $hashFilesDir
sudo $imageCustomizerPath \
  --config-file "$inputConfigFile" \
  --image-file "$inputImage" \
  --build-dir "$buildDir" \
  --output-image-format "$outputFormat" \
  --output-image-file "$verityImage" \
  --output-verity-hashes \
  --output-verity-hashes-dir "$hashFilesDir" \
  --require-signed-rootfs-root-hash \
  --require-signed-root-hashes \
  --log-level "$logLevel"

signedHashFilesDir="./temp/signed-root-hashes"
signedHashFile="$signedHashFilesDir/$(basename $hashFile).sig"

sudo chown $USER:$USER $hashFilesDir
sudo chown $USER:$USER $hashFile

# sign the hash files
cp "$hashFile" "$signedHashFile"
echo "...signed..." > "$signedHashFile"

# inject the file back
signedVerityImage=$outputDir/signed-$outputBaseName
emptyConfig=/home/george/temp/empty-config.yaml
echo "iso:" > $emptyConfig

sudo $imageCustomizerPath \
      --config-file "$emptyConfig" \
      --image-file "$verityImage" \
      --build-dir "$buildDir" \
      --output-image-format "$outputFormat" \
      --output-image-file "$signedVerityImage" \
      --input-signed-verity-hashes-files "$signedHashFile" \
      --log-level "$logLevel"
```

```bash
imageCustomizerPath="./imagecustomizer"
inputConfigFile="./verity-test.yaml"
inputImage="./core-3.0.20241216.vhdx"
buildDir="./build"

keyFile=~./key.pem
certFile=~./cert.pem

outputFormat="qcow2"
outputBaseName="verity-$(date +'%Y%m%d-%H%M').$outputFormat"
outputDir="./output"
verityImage="$outputDir/$outputBaseName"

hashFilesDir="./temp/root-hashes"
hashFile="$hashFilesDir/root.hash"

rm -rf $hashFilesDir
sudo $imageCustomizerPath \
  --config-file "$inputConfigFile" \
  --image-file "$inputImage" \
  --build-dir "$buildDir" \
  --output-image-format "$outputFormat" \
  --output-image-file "$verityImage" \
  --output-verity-hashes \
  --output-verity-hashes-dir "$hashFilesDir" \
  --require-signed-rootfs-root-hash \
  --require-signed-root-hashes \
  --log-level "$logLevel"

echo "Generated: $verityImage"

sudo chown $USER:$USER $unsignedHashFile
sudo chown $USER:$USER $unsignedHashDir

# sign the generated hash
signedHashDir=./root-hashes-signed
signedHashFile=$signedHashDir/root.hash.sig

sudo rm -rf $signedHashDir
mkdir -p $signedHashDir

inputFileStripped=$unsignedHashFile-stripped

rootHash=$(cat $unsignedHashFile)
echo ${rootHash} | tr -d '\n' > $inputFileStripped

openssl smime \
    -sign \
    -nocerts \
    -noattr \
    -binary \
    -in $inputFileStripped \
    -inkey $keyFile \
    -signer $certFile \
    -outform der \
    -out $signedHashFile

# generate the final image
signedVerityImage=$outputDir/signed-$outputBaseName

emptyConfig=./empty-config.yaml
echo "iso:" > $emptyConfig

sudo $imageCustomizerPath \
      --config-file $emptyConfig \
      --image-file $verityImage \
      --build-dir $buildDir \
      --output-image-format $outputFormat \
      --output-image-file $signedVerityImage \
      --input-signed-verity-hashes-files $signedHashFile \
      --log-level $logLevel

  echo "Generated: $signedVerityImage"
```
