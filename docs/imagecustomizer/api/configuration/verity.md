---
parent: Configuration
---

# verity type

Specifies the configuration for dm-verity integrity verification.

Note: Currently root partition (`/`) and usr partition are supported.

Note: The [filesystem](./filesystem.md) item pointing to this verity device, must
include the `ro` option in the [mountPoint.options](./mountpoint.md#options-string).

There are multiple ways to configure a verity enabled image. For
recommendations, see [Verity Image Recommendations](../../concepts/verity.md).

Example:

```yaml
storage:
  bootType: efi
  disks:
  - partitionTableType: gpt
    partitions:
    - id: esp
      type: esp
      size: 8M

    - id: boot
      size: 1G

    - id: root
      size: 2G

    - id: roothash
      size: 100M

    - id: var
      size: 2G

    - id: usr
      size: 1G

    - id: usrhash
      size: 100M

  verity:
  - id: verityroot
    name: root
    dataDeviceId: root
    hashDeviceId: roothash
    corruptionOption: panic

  - id: verityusr
    name: usr
    dataDeviceId: usr
    hashDeviceId: usrhash
    corruptionOption: panic

  filesystems:
  - deviceId: esp
    type: fat32
    mountPoint:
      path: /boot/efi
      options: umask=0077

  - deviceId: boot
    type: ext4
    mountPoint: /boot

  - deviceId: verityroot
    type: ext4
    mountPoint:
      path: /
      options: ro

  - deviceId: verityusr
    type: ext4
    mountPoint:
      path: /usr
      options: ro

  - deviceId: var
    type: ext4
    mountPoint: /var

os:
  bootloader:
    resetType: hard-reset
```

## id [string]

Required.

The ID of the verity object.
This is used to correlate verity objects with
[filesystem](./filesystem.md#deviceid-string) objects.

## name [string]

Required.

The name of the device mapper block device.

The value must be:

- `root` for root partition (i.e. `/`)

## dataDeviceId [string]

The ID of the [partition](./partition.md#id-string) to use as the verity data partition.

## hashDeviceId [string]

The ID of the [partition](./partition.md#id-string) to use as the verity hash partition.

## corruptionOption [string]

Optional.

Specifies how a mismatch between the hash and the data partition is handled.

Supported values:

- `io-error`: Fails the I/O operation with an I/O error.
- `ignore`: Ignores the corruption and continues operation.
- `panic`: Causes the system to panic (print errors) and then try restarting.
- `restart`: Attempts to restart the system.

Default value: `io-error`.
