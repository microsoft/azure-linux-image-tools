---
parent: API
grand_parent: Image Customizer
nav_order: 3
---

# Composable Operating System Image (COSI) Specification

## Revision Summary

| Revision | Spec Date   |
|----------|-------------|
| 1.1      | TBD         |
| 1.0      | 2024-10-09  |

## COSI File Format

The COSI file MUST be an uncompressed tarball. The file extension SHOULD be `.cosi`.

## Tarball Contents

The tarball MUST contain the following files:

- `metadata.json`: A JSON file that contains the metadata of the COSI file.
- Filesystem image files in the folder `images/`: The actual filesystem images
  that will be used to install the OS.

### Layout

The tarball MUST NOT have a common root directory. The metadata file MUST be at
the root of the tarball. If it were extracted with a standard `tar` invocation,
the metadata file would be placed in the current directory.

The metadata file SHOULD be placed at the beginning of the tarball to allow for
quick access to the metadata without having to traverse the entire tarball.

### Partition Image Files

The partition image files are the actual images that reader will use to install
the OS. These MUST be raw partition images.

The partition image files MUST be raw partition images that are compressed using ZSTD
compression.

All partition image files MUST be in the `images` directory or one of its
subdirectories.

### Metadata JSON File

The metadata file MUST be named `metadata.json` and MUST be at the root of the
tarball. The metadata file MUST be a valid JSON file.

#### Schema

##### Root Object

The metadata file MUST contain a JSON object with the following fields:

| Field        | Type                                   | Added in | Required        | Description                                      |
| ------------ | -------------------------------------- | -------- | --------------- | ------------------------------------------------ |
| `version`    | string `MAJOR.MINOR`                   | 1.0      | Yes (since 1.0) | The version of the metadata schema.              |
| `osArch`     | [OsArchitecture](#osarchitecture-enum) | 1.0      | Yes (since 1.0) | The architecture of the OS.                      |
| `osRelease`  | string                                 | 1.0      | Yes (since 1.0) | The contents of `/etc/os-release` verbatim.      |
| `images`     | [Filesystem](#filesystem-object)[]     | 1.0      | Yes (since 1.0) | Filesystem metadata.                             |
| `bootloader` | [Bootloader](#bootloader-object)       | 1.1      | Yes (since 1.1) | Information about the bootloader used by the OS. |
| `osPackages` | [OsPackage](#ospackage-object)[]       | 1.0      | Yes (since 1.1) | The list of packages installed in the OS.        |
| `id`         | UUID (string, case insensitive)        | 1.0      | No              | A unique identifier for the COSI file.           |

If the object contains other fields, readers MUST ignore them. A writer SHOULD
NOT add any other files to the object.

##### `Filesystem` Object

This object carries information about a filesystem and the partition it comes
from in a virtual disk.

| Field        | Type                                 | Added in | Required         | Description                               |
| ------------ | ------------------------------------ | -------- | ---------------- | ----------------------------------------- |
| `image`      | [ImageFile](#imagefile-object)       | 1.0      | Yes (since 1.0)  | Details of the image file in the tarball. |
| `mountPoint` | string                               | 1.0      | Yes (since 1.0)  | The mount point of the filesystem.        |
| `fsType`     | string                               | 1.0      | Yes (since 1.0)  | The filesystem's type. [1]                |
| `fsUuid`     | string                               | 1.0      | Yes (since 1.0)  | The UUID of the filesystem. [2]           |
| `partType`   | UUID (string, case insensitive)      | 1.0      | Yes (since 1.0)  | The GPT partition type. [3] [4] [5]       |
| `verity`     | [VerityConfig](#verityconfig-object) | 1.0      | Conditionally[6] | The verity metadata of the filesystem.    |

_Notes:_

- **[1]** It MUST use the name recognized by the Linux kernel. For example, `ext4` for
    ext4 filesystems, `vfat` for FAT32 filesystems, etc.
- **[2]** It MUST be unique across all filesystems in the COSI tarball.
  Additionally, volumes in an A/B volume pair MUST have unique filesystem UUIDs.
- **[3]** It MUST be a UUID defined by the [Discoverable Partition Specification
    (DPS)](https://uapi-group.org/specifications/specs/discoverable_partitions_specification/)
    when the applicable type exists in the DPS. Other partition types MAY be
    used for types not defined in DPS (e.g., Windows partitions).
- **[4]** The EFI System Partition (ESP) MUST be identified with the UUID
    established by the DPS: `c12a7328-f81f-11d2-ba4b-00a0c93ec93b`.
- **[5]** Should default to `0fc63daf-8483-4772-8e79-3d69d8477de4` (Generic
    Linux Data) if the partition type cannot be determined.
- **[6]** The `verity` field MUST be specified if the OS is configured to open this
    filesystem with `dm-verity`. Otherwise, it MUST be omitted OR set to `null`.

##### `VerityConfig` Object

The `VerityConfig` object contains information required to set up a verity
device on top of a data device.

| Field      | Type                           | Added in | Required        | Description                                              |
| ---------- | ------------------------------ | -------- | --------------- | -------------------------------------------------------- |
| `image`    | [ImageFile](#imagefile-object) | 1.0      | Yes (since 1.0) | Details of the hash partition image file in the tarball. |
| `roothash` | string                         | 1.0      | Yes (since 1.0) | Verity root hash.                                        |

##### `ImageFile` Object

| Field              | Type   | Added in | Required        | Description                                                                               |
| ------------------ | ------ | -------- | --------------- | ----------------------------------------------------------------------------------------- |
| `path`             | string | 1.0      | Yes (since 1.0) | Absolute path of the compressed image file inside the tarball. MUST start with `images/`. |
| `compressedSize`   | number | 1.0      | Yes (since 1.0) | Size of the compressed image in bytes.                                                    |
| `uncompressedSize` | number | 1.0      | Yes (since 1.0) | Size of the raw uncompressed image in bytes.                                              |
| `sha384`           | string | 1.0      | Yes (since 1.1) | SHA-384 hash of the compressed hash image.                                                |

##### `OsArchitecture` Enum

The `osArch` field in the root object MUST be a string that represents the
architecture of the OS. The following table lists the valid values for the
`osArch` field.

| Value    | Description                         |
| -------- | ----------------------------------- |
| `x86_64` | AMD64 or Intel 64-bit architecture. |
| `arm64`  | ARM 64-bit architecture.            |

_Note:_ The `osArch` field uses the names reported by `uname -m` for consistency.
The `osArch` field is case-insensitive.

##### `OsPackage` Object

The `osPackages` field in the root object MUST contain an array of `OsPackage`
objects. Each object represents a package installed in the OS.

| Field     | Type   | Added in | Required        | Description                           |
| --------- | ------ | -------- | --------------- | ------------------------------------- |
| `name`    | string | 1.0      | Yes (since 1.0) | The name of the package.              |
| `version` | string | 1.0      | Yes (since 1.0) | The version of the package installed. |
| `release` | string | 1.0      | Yes (since 1.1) | The release of the package.           |
| `arch`    | string | 1.0      | Yes (since 1.1) | The architecture of the package.      |

##### `Bootloader` Object

| Field         | Type                                     | Added in | Required                         | Description                 |
| ------------- | ---------------------------------------- | -------- | -------------------------------- | --------------------------- |
| `type`        | [`BootloaderType`](#bootloadertype-enum) | 1.1      | Yes (since 1.1)                  | The type of the bootloader. |
| `systemdBoot` | `SystemDBoot`                            | 1.1      | When `type` == `systemd-boot`    | systemd-boot configuration. |

_Notes:_

The `systemd-boot` field is required if the `type` field is set to
`systemd-boot`. It MUST be omitted OR set to `null` if the `type`
field is set to any other value.

##### `BootloaderType` Enum

A string that represents the primary bootloader used in the contained OS. These
are the valid values for the `type` field in the `bootloader` object:

| Value          | Description                                         |
| -------------- | --------------------------------------------------- |
| `systemd-boot` | The system is using systemd-boot as the bootloader. |
| `grub`         | The system is using GRUB as the bootloader.         |

##### `SystemDBoot` Object

This object contains metadata about how systemd-boot is configured in the OS.

| Field     | Type                                             | Added in | Required        | Description                                                                          |
| --------- | ------------------------------------------------ | -------- | --------------- | ------------------------------------------------------------------------------------ |
| `entries` | [`SystemDBootEntry`](#systemdbootentry-object)[] | 1.1      | Yes (since 1.1) | The contents of the `loader/entries/*.conf` files in the systemd-boot EFI partition. |

##### `SystemDBootEntry` Object

This object contains metadata about a specific systemd-boot entry.

| Field     | Type                                                 | Added in | Required        | Description                                            |
| --------- | ---------------------------------------------------- | -------- | --------------- | ------------------------------------------------------ |
| `type`    | [`SystemDBootEntryType`](#systemdbootentrytype-enum) | 1.1      | Yes (since 1.1) | The type of the entry.                                 |
| `path`    | string                                               | 1.1      | Yes (since 1.1) | Absolute path (from the root FS) to the UKI or config. |
| `cmdline` | string                                               | 1.1      | Yes (since 1.1) | The kernel command line.                               |
| `kernel`  | string                                               | 1.1      | Yes (since 1.1) | Kernel release as a string.                            |

##### `SystemDBootEntryType` Enum

A string that represents the type of the systemd-boot entry.

| Value            | Description                                                        |
| ---------------- | ------------------------------------------------------------------ |
| `uki-standalone` | The entry is a bare UKI file in the ESP.                           |
| `uki-config`     | The entry is a config file with a UKI.                             |
| `config`         | The entry is a config file with a kernel, initrd and command line. |

#### Samples

##### Simple Image

```json
{
    "version": "1.1",
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
    "osRelease": "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0.20240824\"\nID=azurelinux\nVERSION_ID=\"3.0\"\nPRETTY_NAME=\"Microsoft Azure Linux 3.0\"\nANSI_COLOR=\"1;34\"\nHOME_URL=\"https://aka.ms/azurelinux\"\nBUG_REPORT_URL=\"https://aka.ms/azurelinux\"\nSUPPORT_URL=\"https://aka.ms/azurelinux\"\n",
    "bootloader": {
        "type": "grub"
    },
    "osPackages": [
        {
            "name": "bash",
            "version": "5.1.8",
            "release": "1.azl3",
            "arch": "x86_64"
        },
        {
            "name": "coreutils",
            "version": "8.32",
            "release": "1.azl3",
            "arch": "x86_64"
        },
        {
            "name": "systemd",
            "version": "255",
            "release": "20.azl3",
            "arch": "x86_64"
        },
        // More packages...
    ]
}
```

##### Verity Image with UKI

```json
{
    "version": "1.1",
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
    "osRelease": "NAME=\"Microsoft Azure Linux\"\nVERSION=\"3.0.20240824\"\nID=azurelinux\nVERSION_ID=\"3.0\"\nPRETTY_NAME=\"Microsoft Azure Linux 3.0\"\nANSI_COLOR=\"1;34\"\nHOME_URL=\"https://aka.ms/azurelinux\"\nBUG_REPORT_URL=\"https://aka.ms/azurelinux\"\nSUPPORT_URL=\"https://aka.ms/azurelinux\"\n",
    "bootloader": {
        "type": "systemd-boot",
        "systemdBoot": {
            "entries": [
                {
                    "type": "uki-standalone",
                    "path": "/boot/efi/EFI/Linux/azurelinux-uki.efi",
                    "cmdline": "root=/dev/disk/by-partuuid/88d2fa9b-7a32-450a-a9f8-aa9c3de79298 ro",
                    "kernel": "6.6.78.1-3.azl3"
                }
            ]
        }
    },
    "osPackages": [
        {
            "name": "systemd",
            "version": "255",
            "release": "20.azl3",
            "arch": "x86_64"
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
- Tar is a super simple tape format. It is just a stream of files with metadata
  at the beginning. This makes it easy to read and write.

**Why an uncompressed tarball?**

- This allows the metadata file to be easily read without needing to decompress and
  extract the entire tarball. Also, compressing the tarball doesn't provide any
  meaningful size reductions since the partition images are all compressed individually.

**Why not ZIP?**

- ZIP is more complex than tar. It has more features, notably an index at the
  end of the file. However, to compute the hash of the file, we need to read it
  through, anyway, so we can index the file as we read it. Even in cases where
  we don't need to compute the hash, to take full advantage of the index, we
  would need to implement our own ZIP reader.
- ZSTD support in ZIP is not very
  widespread.

**Why not use a custom format?**

- Making a custom format MAY help us achieve greater performance is some edge
  cases, specifically network streaming. However, the complexity of creating and
  maintaining a custom format outweighs the benefits. Tar is simple and good
  enough for our needs.

**Why not use VHD or VHDX?**

- VHD and VHDX are complex formats that are not designed for our use case. They
  are designed to be used as virtual disks, not as a simple container for
  partition images. They are also not as portable as tarballs.
- They do not have a standard way to store metadata. The spec does include some
  empty space reserved for future expansion, but using it would require us to
  implement our own fork of the VHD/VHDX spec.

  **What about a VHD+Metadata?**

  - Putting the metadata in a separate file would defeat the purpose of having a
    single file.

**What other formats were considered?**

- We considered using a custom format, but the complexity of creating and
  maintaining a custom format outweighs the benefits.
- SquashFS was considered, but it would only change the container around the
  filesystems images. When considering only the container, there was no real
  practical benefit to using SquashFS over Tar.
