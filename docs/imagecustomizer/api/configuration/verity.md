---
parent: Configuration
---

# verity type

Specifies the configuration for dm-verity integrity verification.

Note: Currently root partition (`/`) and usr partition (`/usr`) are supported.

Note: The [filesystem](./filesystem.md) item pointing to this verity device, must
include the `ro` option in the [mountPoint.options](./mountpoint.md#options-string).

There are multiple ways to configure a verity enabled image. For
recommendations, see [Verity Image Recommendations](../../concepts/verity.md).

Example of enabling root Verity:

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

  verity:
  - id: verityroot
    name: root
    dataDeviceId: root
    hashDeviceId: roothash
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

  - deviceId: var
    type: ext4
    mountPoint: /var

os:
  bootloader:
    resetType: hard-reset
```

Example of enabling usr Verity:

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

    - id: usr
      size: 2G

    - id: usrhash
      size: 100M

  verity:
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

  - deviceId: root
    type: ext4
    mountPoint: /

  - deviceId: verityusr
    type: ext4
    mountPoint:
      path: /usr
      options: ro

os:
  bootloader:
    resetType: hard-reset
```

Example of enabling verity on existing partitions in the base image:

```yaml
storage:
  verity:
  - id: verityusr
    name: usr
    dataDevice:
      idType: part-label
      id: usr
    hashDevice:
      idType: part-label
      id: usrhash
    corruptionOption: panic
```

Added in v0.7.

## id [string]

Required.

The ID of the verity object.
This is used to correlate verity objects with
[filesystem](./filesystem.md#deviceid-string) objects.

Added in v0.7.

## name [string]

Required.

The name of the device mapper block device.

The value must be:

- `root` for root partition (i.e. `/`).

  Added in v0.7.
  
- `usr` for the usr partition (i.g. `/usr`).

  Added in v0.11.

Added in v0.7.

## dataDevice [[verityPartition](./verityPartition.md)]

The existing partition within the base image to use as the verity data partition.

Must be used with `hashDevice`.

Added in v0.13.

## dataDeviceId [string]

The ID of the new [partition](./partition.md#id-string) to use as the verity data
partition.

Must be used with `hashDeviceId`.

Added in v0.7.

## dataDeviceMountIdType [string]

How the verity data partition is referenced in the OS. For example, within the
`/etc/fstab` and within the kernel command-line args.

Supported values:

- `uuid`: Identify the partition by filesystem UUID.
- `part-uuid`: Identify the partition by partition UUID.
- `part-label`: Identify the partition by partition label.

Added in v0.7.

## hashDevice [[verityPartition](./verityPartition.md)]

The existing partition within the base image to use as the verity hash partition.

Must be used with `dataDevice`.

Added in v0.13.

## hashDeviceId [string]

The ID of the new [partition](./partition.md#id-string) to use as the verity hash
partition.

Must be used with `dataDeviceId`.

Added in v0.7.

## hashDeviceMountIdType [string]

How the verity hash partition is referenced in the OS. For example, within the
`/etc/fstab` and within the kernel command-line args.

Supported values:

- `uuid`: Identify the partition by filesystem UUID.
- `part-uuid`: Identify the partition by partition UUID.
- `part-label`: Identify the partition by partition label.

Added in v0.7.

## corruptionOption [string]

Optional.

Specifies how a mismatch between the hash and the data partition is handled.

Supported values:

- `io-error`: Fails the I/O operation with an I/O error.
- `ignore`: Ignores the corruption and continues operation.
- `panic`: Causes the system to panic (print errors) and then try restarting.
- `restart`: Attempts to restart the system.

Default value: `io-error`.

Added in v0.7.

## hashSignaturePath [string]

Optional.

Specifies the path where the signed verity hash file should be injected into the
image. This path is typically used by the `systemd-veritysetup` module to verify
the verity hash against a signature at boot time.

This path **must be located under the boot partition**. (This restriction may be
lessened in the future.) For example, if the boot partition is mounted at
`/boot`, then `hashSignaturePath: /boot/root.hash.sig` will result in a
destination of `/root.hash.sig` relative to the boot partition during injection.

When this field is specified, Image Customizer will output the corresponding unsigned hash
file (`verity-hash`) as an artifact if the
[`output.artifacts`](./outputArtifacts.md) API is configured.

The generated `inject-files.yaml` will include an entry to inject the signed
hash file to the specified path inside the boot partition.

If `hashSignaturePath` is not configured for a given Verity entry, the verity
hash file will not be output even if `verity-hash` is listed in the
[`output.artifacts.items`](./outputArtifacts.md#items-string). Only Verity
entries with `hashSignaturePath` defined will produce a `verity-hash` artifact.

Added in v0.16.
