---
title: systemd-growfs-root
parent: Verity Image Recommendations
---

# Verity and systemd-growfs-root

This service attempts to resize the root filesystem, which fails since verity
makes the root filesystem readonly and a fixed size.

## Solution 1: Do nothing

Since the root filesystem is readonly, the `systemd-growfs-root` service will
fail. However, the only impact will be an error in the boot logs.

## Solution 2: Disable service

Disabling the service removes the error from the boot logs.

```yaml
os:
  services:
    disable:
    - systemd-growfs-root
```
