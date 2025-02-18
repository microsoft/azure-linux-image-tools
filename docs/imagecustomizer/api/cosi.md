---
parent: API
nav_order: 3
---

# Composable Operating System Image (COSI) Specification

## COSI File Format

The COSI file MUST be an uncompressed tarball. The file extension SHOULD be `.cosi`.

## Tarball Contents

The tarball MUST contain the following files:

- `metadata.json`: A JSON file that contains the metadata of the COSI file.
- Partition image files in the folder `images`: The actual partition images
  that will be used to install the OS.

If the tarball contains other files, readers MUST ignore them. A writer SHOULD NOT
add any other files to the tarball.

The tarball MUST NOT have a common root directory. The `metadata.json` file and the
`images` directory MUST be in the root directory of the tarball.

The metadata file SHOULD be placed at the beginning of the tarball to allow for
quick access to the metadata without needing to traverse the entire tarball.

## Partition Image Files

The partition image files MUST be raw partition images that are compressed using ZSTD
compression.

All partition image files MUST be in the `images` directory or one of its
subdirectories.

## Metadata JSON File

The metadata file MUST be named `metadata.json` and MUST be a valid JSON file.

## Metadata JSON Schema

### Root Object

The metadata file MUST contain a JSON object with the following fields:

| Field        | Type                                   | Required | Description                                            |
| ------------ | -------------------------------------- | -------- | ------------------------------------------------------ |
| `version`    | string `MAJOR.MINOR`                   | Yes      | The version of the metadata schema. MUST be `1.0`.     |
| `osArch`     | [OsArchitecture](#osarchitecture-enum) | Yes      | The CPU architecture of the OS.                        |
| `osRelease`  | string                                 | Yes      | The contents of OS's `/etc/os-release` file.           |
| `images`     | [Image](#image-object)[]               | Yes      | Metadata of partition images that contain filesystems. |
| `osPackages` | [OsPackage](#ospackage-object)[]       | No       | The list of packages installed in the OS.              |
| `id`         | UUID (string, case insensitive)        | No       | A unique identifier for the COSI file.                 |

If the object contains other fields, readers MUST ignore them. A writer SHOULD NOT
add any other files to the object.

### `Image` Object

| Field        | Type                                 | Required | Description                               |
| ------------ | ------------------------------------ | -------- | ----------------------------------------- |
| `image`      | [ImageFile](#imagefile-object)       | Yes      | Details of the image file in the tarball. |
| `mountPoint` | string                               | Yes      | The mount point of the partition.         |
| `fsType`     | string                               | Yes      | The filesystem type of the partition. [1] |
| `fsUuid`     | string                               | Yes      | The UUID of the filesystem.               |
| `partType`   | UUID (string, case insensitive)      | Yes      | The GPT partition type. [2] [3] [4]       |
| `verity`     | [VerityConfig](#verityconfig-object) | No       | The verity metadata of the partition.     |

_Notes:_

- **[1]** It MUST use the name recognized by the Linux kernel. For example, `ext4` for
    ext4 filesystems, `vfat` for FAT32 filesystems, etc.

- **[2]** It MUST be a UUID defined by the [Discoverable
    Partition Specification
    (DPS)](https://uapi-group.org/specifications/specs/discoverable_partitions_specification/)
    when the applicable type exists in the DPS. Other partition types MAY be
    used for types not defined in DPS (e.g. Windows partitions).

- **[3]** The EFI Sytem Partition (ESP) MUST be identified with the UUID
    established by the DPS: `c12a7328-f81f-11d2-ba4b-00a0c93ec93b`.

- **[4]** Should default to `0fc63daf-8483-4772-8e79-3d69d8477de4` (Generic
    Linux Data) if the partition type cannot be determined.

### `VerityConfig` Object

The `VerityConfig` object contains information required to set up a verity
device on top of a data partition.

| Field      | Type                           | Required | Description                                              |
| ---------- | ------------------------------ | -------- | -------------------------------------------------------- |
| `image`    | [ImageFile](#imagefile-object) | Yes      | Details of the hash partition image file in the tarball. |
| `roothash` | string                         | Yes      | Verity root hash.                                        |

### `ImageFile` Object

| Field              | Type   | Required | Description                                                                               |
| ------------------ | ------ | -------- | ----------------------------------------------------------------------------------------- |
| `path`             | string | Yes      | Absolute path of the compressed image file inside the tarball. MUST start with `images/`. |
| `compressedSize`   | number | Yes      | Size of the compressed image in bytes.                                                    |
| `uncompressedSize` | number | Yes      | Size of the raw uncompressed image in bytes.                                              |
| `sha384`           | string | No[5]    | SHA-384 hash of the compressed hash image.                                                |

_Notes:_

- **[5]** The `sha384` field is optional, but it is RECOMMENDED to include it for
    integrity verification.

### `OsArchitecture` Enum

The `osArch` field in the root object MUST be a string that represents the
architecture of the OS. The following table lists the valid values for the
`osArch` field.

| Value    | Description                         |
| -------- | ----------------------------------- |
| `x86_64` | AMD64 or Intel 64-bit architecture. |
| `arm64`  | ARM 64-bit architecture.            |

### `OsPackage` Object

When present, the `osPackages` field in the root object MUST contain an array of
`OsPackage` objects. Each object represents a package installed in the OS.

A reader MAY use this field to determine if the OS is missing any packages that are
required for how the user intends to use the OS image.

| Field     | Type   | Required | Description                           |
| --------- | ------ | -------- | ------------------------------------- |
| `name`    | string | Yes      | The name of the package.              |
| `version` | string | Yes      | The version of the package installed. |
| `release` | string | No       | The release number of the package.    |
| `arch`    | string | No       | The CPU architecture of the package.  |

### Samples

#### Simple Image

```json
{
    "version": "1.0",
    "images": [
        {
            "image": {
                "path": "images/esp.rawzst",
                "compressedSize": 839345,
                "uncompressedSize": 8388608,
                "sha384": "2decc64a828dbbb76779731cd4afd3b86cc4ad0af06f4afe594e72e62e33e520a6649719fe43f09f11d518e485eae0db"
            },
            "mountPoint": "/boot/efi",
            "fsType": "vfat",
            "fsUuid": "C3D4-250D",
            "partType": "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", // <-- ESP DPS GUID
            "verity": null
        },
        {
            "image": {
                "path": "images/root.rawzst",
                "compressedSize": 192874245,
                "uncompressedSize": 899494400,
                "sha384": "98ea4adbbb8ce0220d109d53d65825bd5a565248e4af3a9346d088918e7856ac2c42e13461cac67dbf3711ff69695ec3"
            },
            "mountPoint": "/",
            "fsType": "ext4",
            "fsUuid": "88d2fa9b-7a32-450a-a9f8-aa9c3de79298",
            "partType": "root",
            "verity": null
        }
    ],
    "osRelease": "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0.20240824\"\nID=azurelinux\nVERSION_ID=\"3.0\"\nPRETTY_NAME=\"Microsoft Azure Linux 3.0\"\nANSI_COLOR=\"1;34\"\nHOME_URL=\"https://aka.ms/azurelinux\"\nBUG_REPORT_URL=\"https://aka.ms/azurelinux\"\nSUPPORT_URL=\"https://aka.ms/azurelinux\"\n"
}
```

#### Verity Image

```json
{
    "version": "1.0",
    "images": [
        {
            "image": {
                "path": "images/root.rawzst",
                "compressedSize": 192874245,
                "uncompressedSize": 899494400,
                "sha384": "98ea4adbbb8ce0220d109d53d65825bd5a565248e4af3a9346d088918e7856ac2c42e13461cac67dbf3711ff69695ec3"
            },
            "mountPoint": "/",
            "fsType": "ext4",
            "fsUuid": "88d2fa9b-7a32-450a-a9f8-aa9c3de79298",
            "partType": "4f68bce3-e8cd-4db1-96e7-fbcaf984b709", // <-- Root amd64/x86_64 DPS GUID
            "verity": {
                "image": {
                    "path": "images/root-verity.rawzst",
                    "compressedSize": 26214400,
                    "uncompressedSize": 524288000,
                    "sha384": "51356c53fbdd5c196395ccd389116f2e7769443cb4e945bc9b6bc3c805cf857c375df010469f8f45ef0c5b07456b023d"
                },
                "roothash": "646c82fa4c3f97e6cddc3996315c7f04b2beb721fb24fa38835136492a84eb19"
            }
        },
        // More images...
    ],
    "osRelease": "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0.20240824\"\nID=azurelinux\nVERSION_ID=\"3.0\"\nPRETTY_NAME=\"Microsoft Azure Linux 3.0\"\nANSI_COLOR=\"1;34\"\nHOME_URL=\"https://aka.ms/azurelinux\"\nBUG_REPORT_URL=\"https://aka.ms/azurelinux\"\nSUPPORT_URL=\"https://aka.ms/azurelinux\"\n"
}
```

#### Packages

```json
{
    "version": "1.0",
    "images": [
        // Images...
    ],
    "osRelease": "<OS_RELEASE>",
    "osPackages": [
        {
            "name": "bash",
            "version": "5.1.8"
        },
        {
            "name": "coreutils",
            "version": "8.32"
        },
        {
            "name": "systemd",
            "version": "255"
        },
        // More packages...
    ]
}
```

## FAQ and Notes

**Why tar?**

- Tar is simple and ubiquitous. It is easy to create and extract tarballs on
  virtually any platform. There are native libraries for virtually every
  programming language to handle tarballs, including Rust and Go.

**Why an uncompressed tarball?**

- This allows the metadata file to be easily read without needing to decompress and
  extract the entire tarball. Also, compressing the tarball doesn't provide any
  meaningful size reductions since the partition images are all compressed individually.

**Why not use a custom format?**

- Making a custom format MAY help us achieve greater performance is some edge
  cases, specifically network streaming. However, the complexity of creating and
  maintaining a custom format outweighs the benefits. Tar is simple and good
  enough for our needs.
