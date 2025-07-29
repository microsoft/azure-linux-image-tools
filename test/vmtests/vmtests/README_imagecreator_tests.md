# Image Creator Tests

This directory contains VM tests for the Image Creator tool, similar to the existing Image Customizer tests.

## Overview

The Image Creator tests create new Azure Linux images from scratch using the Image Creator binary tool, then boot them in VMs to verify they work correctly.

## Test Structure

- `test_imagecreator.py` - Main test file containing the Image Creator VM tests
- `utils/imagecreator.py` - Utility functions for running the Image Creator binary

## Key Differences from Image Customizer Tests

1. **No Input Images**: Image Creator creates images from scratch, so there are no input image fixtures
2. **No Docker**: Image Creator is a binary tool, not a containerized tool like Image Customizer
3. **RPM Sources Required**: Tests need `--rpm-sources` to specify package repositories
4. **Tools Tar Required**: Tests need `--tools-tar` to specify the tools tarball
5. **Limited Output Formats**: Image Creator supports fewer output formats (no ISO)

## Test Functions

- `test_create_image_efi_qcow_output()` - Creates a QCOW2 image with EFI boot
- `test_create_image_efi_raw_output()` - Creates a RAW image with EFI boot  

## Configuration

Tests use the `minimal-os.yaml` configuration file from the Image Creator library testdata directory. This creates a minimal Azure Linux 3.0 image with essential packages.

## Running the Tests

```bash
pytest test_imagecreator.py \
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

1. Creates a new image using the Image Creator binary
2. Creates a differencing disk for VM testing
3. Boots the image in a libvirt VM
4. Connects via SSH and runs basic validation:
   - Checks that the OS is Azure Linux 3.0
   - Verifies essential packages are installed (kernel, systemd, grub2, bash)
   - Validates the system can boot and respond to commands
