---
title: cloud-init
parent: Verity Image Recommendations
---

# Verity and cloud-init

cloud-init has various features to configure the system (e.g., user accounts,
networking, etc.), but many of these require the /etc directory to be writable.
In verity-protected images with a read-only root filesystem, cloud-init cannot
perform these configurations effectively.

## Solution: Disable cloud-init

Given the limitations, the general recommendation is to disable cloud-init in
verity images to prevent potential issues.

```yaml
os:
  services:
    disable:
    - cloud-init
```
