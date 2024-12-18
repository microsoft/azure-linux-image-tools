# verity type

Specifies the configuration for dm-verity integrity verification.

Note: Currently only root partition (`/`) is supported. Support for other partitions
(e.g. `/usr`) may be added in the future.

Note: The [filesystem](#filesystem-type) item pointing to this verity device, must
include the `ro` option in the [mountPoint.options](#options-string).

There are multiple ways to configure a verity enabled image. For
recommendations, see [Verity Image Recommendations](./verity.md).

## id [string]

Required.

The ID of the verity object.
This is used to correlate verity objects with [filesystem](#filesystem-type)
objects.


## name [string]

Required.

The name of the device mapper block device.

The value must be:

- `root` for root partition (i.e. `/`)

## dataDeviceId [string]

The ID of the [partition](#partition-type) to use as the verity data partition.

## hashDeviceId [string]

The ID of the [partition](#partition-type) to use as the verity hash partition.

## corruptionOption [string]

Optional.

Specifies how a mismatch between the hash and the data partition is handled.

Supported values:

- `io-error`: Fails the I/O operation with an I/O error.
- `ignore`: Ignores the corruption and continues operation.
- `panic`: Causes the system to panic (print errors) and then try restarting.
- `restart`: Attempts to restart the system.

Default value: `io-error`.
