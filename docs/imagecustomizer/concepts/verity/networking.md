---
title: Networking
parent: Verity Image Recommendations
sidebar_position: 3
---

# Verity and Networking

In non-verity images, usually user can leverage cloud-init to provide default
networking settings. However, cloud-init fails to provision the network in
verity images since /etc is not writable.

## Solution: Specify Network Settings Manually

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
