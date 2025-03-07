---
title: var Partition
parent: Verity Image Recommendations
---

# Verity and `/var` Partition

Many services  (e.g., auditd, docker, logrotate, etc.) require write access to
the /var directory.

## Solution: Create a Writable Persistent /var Partition

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