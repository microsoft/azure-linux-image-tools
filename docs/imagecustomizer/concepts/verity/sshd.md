---
title: sshd
parent: Verity Image Recommendations
sidebar_position: 4
---

# Verity and sshd

The `sshd` service requires write access to the SSH host keys, which by default
are stored in `/etc/ssh`. However, with the root filesystem being read-only,
this prevents `sshd` from running correctly.

## Solution: Create a writable persistent partition and redirect SSH host keys

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
