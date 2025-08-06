# Image Creator Tests

This directory contains VM tests for the Image Creator tool, similar to the existing Image
Customizer tests in the parent directory.

## Overview

The Image Creator tests create new Azure Linux images from scratch using the Image Creator binary
tool, then boot them in VMs to verify they work correctly.

## Test Structure

- `test_imagecreator.py` - Main test file containing the Image Creator VM tests
- `conftest.py` - Pytest configuration and fixtures specific to Image Creator tests
- `../utils/imagecreator.py` - Utility functions for running the Image Creator binary

**Configuration Files:**

- SSH base config: `toolkit/tools/pkg/imagecreatorlib/testdata/ssh-base-config.yaml` - Base
  ImageCustomizer configuration used with `add_ssh_to_config()` function to add SSH access and
  VM-friendly settings

## Key Differences from Image Customizer Tests

1. **No Input Images**: Image Creator creates images from scratch, so there are no input image
   fixtures
2. **No Docker**: Image Creator is a binary tool, not a containerized tool like Image Customizer
3. **RPM Sources Required**: Tests need `--rpm-sources` to specify package repositories
4. **Tools Tar Required**: Tests need `--tools-tar` to specify the tools tarball
5. **Limited Output Formats**: Image Creator supports fewer output formats (no ISO)

## Test Functions

- `test_create_image_efi_qcow_output()` - Creates a QCOW2 image with EFI boot
- `test_create_image_efi_raw_output()` - Creates a RAW image with EFI boot  

## Configuration

Tests use the `minimal-os.yaml` configuration file from the Image Creator library testdata directory
for creating the base image. SSH access and VM-friendly settings are added using the
`ssh-base-config.yaml` file with the `add_ssh_to_config()` function.

## Running the Tests

```bash
# From the vmtests directory
pytest imagecreator/test_imagecreator.py \
  --image-creator-binary-path <path-to-imagecreator-binary> \
  --rpm-sources <path-to-rpm-repo> \
  --tools-tar <path-to-tools.tar.gz> \
  --ssh-private-key <path-to-ssh-key> \
  --logs-dir <path-to-logs>
```

## Building the Image Creator Binary

Before running tests, you need to build the Image Creator binary:

```bash
sudo make -C ./toolkit go-imagecreator
```

The binary will be located at `./toolkit/out/tools/imagecreator`.

## Test Validation

Each test:

1. Creates a new image using the Image Creator binary with `minimal-os.yaml`
2. Customizes the image with SSH access using ImageCustomizer and the `add_ssh_to_config()` function
   with `ssh-base-config.yaml`
3. Creates a differencing disk for VM testing
4. Boots the image in a libvirt VM
5. Connects via SSH and runs basic validation:
   - Checks that the OS is Azure Linux 3.0
   - Verifies essential packages are installed (kernel, systemd, grub2, bash)
   - Validates the system can boot and respond to commands
   