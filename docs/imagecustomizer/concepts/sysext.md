---
parent: Concepts
nav_order: 6
---

# System extension

System extensions (sysext) are a systemd feature that allows extending an operating
system without modifying the base system. They are useful for immutable or read-only
OS environments, enabling modular functionality while preserving system integrity.

Key Characteristics:
- Dynamic Overlay: Files from a sysext image are dynamically overlaid onto /usr and /opt
  at runtime.
- Immutable Base System: The base OS remains unchanged, allowing independent updates.
- Modular & Flexible: Extensions can be added, removed, or updated without modifying
  core OS files.
- Verification & Security: Sysext images can include dm-verity and cryptographic
  signatures to ensure integrity and authenticity.
- No Persistent Changes: Once deactivated, system extensions disappear from the
  filesystem without leaving traces.

# System Extension Image

System extension images are the packaged format used to deliver system extensions.
They contain the files to be overlaid onto the base system, typically under `/usr` and
`/opt`.

System extensions may be delivered in several formats:
- Plain directories or btrfs subvolumes containing the OS tree
- Disk images with a GPT disk label, following the Discoverable Partitions Specification
- Raw disk images without a partition table, using a naked Linux file system such as
  erofs, squashfs, or ext4

This doc focuses on GPT-labeled disk images because they offer strong standardization,
compatibility with the Discoverable Partitions Specification, and support for integrity
features like `dm-verity`. This makes them ideal for production environments where
immutability, verifiability, and consistent system behavior are critical.

A properly formatted sysext image will typically contain:

| Partition Name                     | Description                                       |
|------------------------------------|---------------------------------------------------|
| **Root Filesystem Partition**      | Contains the extension's files (e.g., binaries,   |
|                                    | libraries, configurations).                        |
| **Verity Hash Partition**          | Stores a Merkle tree hash of the root filesystem, |
| **(Optional)**                     | enabling dm-verity for integrity checks.          |
| **Signature Partition**            | Holds a digital signature verifying the integrity |
| **(Optional)**                     | of the hash data.                                 |

# Building Sysext Images with mkosi

mkosi is the recommended tool for building system extension images. It automates the
creation of properly formatted GPT images, including:
- Partitioning the image (root filesystem, verity, signature).
- Applying integrity verification using dm-verity.
- Signing the image (if needed).

For detailed documentation on mkosi commands and configuration, refer to 
[doc](https://github.com/systemd/mkosi/blob/main/mkosi/resources/man/mkosi.1.md)

## 1. Install mkosi

Clone the latest version from GitHub (recommended):
```
git clone https://github.com/systemd/mkosi.git
cd mkosi
./bin/mkosi --version
```

Or install via package manager.

## 2. Define the Image Configuration

1. Create a `mkosi.conf`: defines the build parameters for your sysext image. This
   configuration file tells mkosi what to include in the image, how to format it, and how
   to handle verification.

2. Update `sysext.repart.d/` if needed: this directory contains partition definition
   files (.conf files) that instruct systemd-repart how to create and structure the
   partitions within the sysext image, including:
    - Content partition (holds the actual files)
    - Verity hash partition (used for integrity verification)
    - Signature partition (optional, contains cryptographic signature)

   Typically, users can create customized sysext.repart.d/ files for certain aspects of
   the image structure. However, when using Format=sysext - unlike Format=disk, you
   cannot completely replace these definitions with your own custom repart
   configurations. **For format=sysext, mkosi is specifically using its own predefined
   repart definitions located at 
   [mkosi/resources/repart/definitions/sysext.repart.d](https://github.com/systemd/mkosi/tree/e4be13971af896ac6301b78cb045fb4cbe7a2b04/mkosi/resources/repart/definitions/sysext.repart.d).
   If you need to modify the partition structure (e.g., to change filesystem types or
   partition sizes), you'll need to edit the .conf files in the mkosi installation
   directory, which will be in the system paths if installed via package manager or in
   the cloned repository.**

Example mkosi.conf

```ini
[Output]
Format=sysext
ImageId=kubernetes
ImageVersion=1.30.7
OutputDirectory=mkosi.output

[Validation]
Verity=signed
VerityKey=verity_key.pem
VerityCertificate=verity_cert.crt

[Content]
ExtraTrees=/usr/local/bin/kubectl:/usr/local/bin/kubectl, /usr/local/bin/kubelet:/usr/local/bin/kubelet
```

### [Output] Section
- Format=sysext: Specifies that we are building a system extension image.
- ImageId=kubernetes: Defines the image identifier, which is used in the output filename.
- ImageVersion=1.30.7: Sets the version number of the image, included in metadata and
  the default filename.
- OutputDirectory=mkosi.output: Specifies the directory where the output file will be
  stored.

### [Validation] Section
The Verity= setting determines how your sysext image is secured:
- signed: Fully signed image with both hash data and a cryptographic signature (requires
  VerityKey & VerityCertificate).
- defer: Allocates space for a signature but does not populate it yet (useful for
  external signing).
- hash: Hash verification only (no signature).
- false: No verity at all (unsigned, unverified image).
- VerityKey=verity_key.pem: The private key used for signing.
- VerityCertificate=verity_cert.crt: The public certificate used for verification.

### [Content] Section

The `ExtraTrees=` setting specifies which binaries or files to include in the `sysext`
image.
Format:
`source_path:destination_path` (comma-separated for multiple entries)

## Build and validate the Image
```
./bin/mkosi --force
```

When a sysext image is built with the configurations above, it contains three partitions:

| Device            | Start Sector | End Sector | Size  | Type                       |
|-------------------|--------------|------------|-------|----------------------------|
| kubernetes.raw1   | 2048         | 79983      | 38.1M | Linux root (x86-64)        |
| kubernetes.raw2   | 79984        | 100463     | 10M   | Linux root verity (x86-64) |
| kubernetes.raw3   | 100464       | 100495     | 16K   | Linux root verity sign. (x86-64)|

- kubernetes.raw1: main filesystem partition (contains kubectl and kubelet)
- kubernetes.raw2: contains the Merkle tree hash data used to verify the integrity of
  the main filesystem partition
- kubernetes.raw3: contains the verity signature

# Integrate Sysext image into base image through Prism

As part of its image customization process, Prism can include the .raw image into
/var/lib/extensions/. The public certificate verity_cert.crt can be placed under /etc/verity.d/
if you'd like for persistent certificate storage across reboots, or be copied to /run/verity.d/
at runtime for temporary certificate storage that will be cleared after system restart.

To have your sysext extensions automatically applied at boot time, ensure
systemd-sysext.service is active so that `systemd-sysext merge` is triggered automatically
at boot.

Example Prism config.yaml:
```yaml
# config.yaml
os:
  additionalFiles:
  - source: kubernetes.raw
    destination: /var/lib/extensions/kubernetes.raw
  - source: verity_cert.crt
    destination: /etc/verity.d/verity_cert.crt 

scripts:
  postCustomization:
  - content: |
      systemctl enable systemd-sysext.service
      systemctl start systemd-sysext.service
```

Systemd Verification Process at Runtime:

- `systemd-sysext merge` is triggered either manually or automatically at boot.
- systemd checks for a signature partition in the sysext image:
    - It extracts the embedded public key.
    - It compares it with the trusted certs in the base image.
    - If the certificate matches, systemd verifies the hash using dm-verity.
- Sysext is merged into /usr and /opt.

To check whether a system extension has been successfully loaded:

```
systemd-sysext status
```

# What if Prism and sysext create overlay on the same mount point?

Prism has an API for overlay creation, so there could be a case where Prism and sysext create overlay on
the same mount point. Here is what will happen:

Prism creating an overlay first (e.g. for /usr or /opt)
systemd-sysext trying to create another overlay on the same mount point

When systemd-sysext detects this situation, it automatically adds the redirect_dir=on option
to allow its overlay to function without fully merging with the existing one. This is a way to
stack its overlay on top of the existing overlay. It "redirects" directory lookup operations to
the appropriate layer rather than merging directory contents. 

Binaries and files would not be visible in the PATH lookup. You can either 
use full paths to access the binaries directly
(e.g., /usr/local/bin/kubectl instead of just kubectl), or create symbolic links from a 
directory that is properly in your PATH to the actual binary locations.

If Prism doesn't create overlays on /usr or /opt, then systemd-sysext would be
the only system creating overlays on these directories, and there would be no need for the
redirect_dir=on option. These overlays would function normally, without redirection
Binaries and files would be properly visible in the PATH lookup.