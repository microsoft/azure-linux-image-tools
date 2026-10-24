---
parent: Image Customizer
nav_order: 5
---

# Developers guide

## Prerequisites

- Golang
- [Trivy](https://github.com/aquasecurity/trivy/releases/latest) for license scanning

For other package dependencies, see [Getting started guide](./quick-start/quick-start.md)

## Build Image Customizer binary

Run:

```bash
sudo make -C ./toolkit go-imagecustomizer
```

### Experimental ACL root-verity build

The `acl_root_verity` Go build tag reserves support for future ACL root-verity
conversion. This foundation does not add conversion behavior or configuration fields.
Normal builds reject the `acl-root-verity` preview feature; tagged builds accept it
but do not change image processing.

```console
make -C toolkit go-imagecustomizer-acl-root-verity
```

This target combines `prod,acl_root_verity` and writes the separately named
`toolkit/out/tools/imagecustomizer-acl-root-verity` binary. Its `--version` output
identifies the build feature. Normal build and release targets remain unchanged.

Future conversion implementations and registration must live in
`//go:build acl_root_verity` files, with rejecting stubs in `!acl_root_verity` files.
Before any conversion work, library entry points must call `RequireAclRootVerity`
with the resolved preview features. The build tag alone is not runtime opt-in.
Conversion API and schema changes belong in a later change.

CI checks default, `prod`, `acl_root_verity`, and `prod,acl_root_verity` builds on
both architectures. These are focused gate checks; the existing full API suite
continues to run without `prod`.

### Update the API schema

If you're updating the image customizer API as part of your change, you need to
update the API `schema.json` before sending a PR. To do so, run:

```bash
make -C toolkit/tools/imagecustomizerschemacli/
```

## Run toolkit tests

Run:

```bash
sudo go test -C ./toolkit/tools ./...
```

## Run Image Customizer specific tests

1. Build (or download) the vhdx/vhd image files for:

   - [Azure Linux 2.0 core-efi](https://github.com/microsoft/azurelinux/blob/2.0/toolkit/imageconfigs/core-efi.json)
   - [Azure Linux 3.0 core-efi](https://github.com/microsoft/azurelinux/blob/3.0/toolkit/imageconfigs/core-efi.json)
   - Azure Linux 4.0 core-efi
   - [Ubuntu 22.04 Azure cloud](https://cloud-images.ubuntu.com/releases/22.04/release/) (download the `*-azure.vhd.tar.gz` and extract)
   - [Ubuntu 24.04 Azure cloud](https://cloud-images.ubuntu.com/releases/24.04/release/) (download the `*-azure.vhd.tar.gz` and extract)

2. Download the test RPM files:

   ```bash
   ./toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -d azurelinux -t 3.0
   ```

3. Run the tests:

   ```bash
   AZURE_LINUX_2_CORE_EFI_VHDX="<core-efi-2.0.vhdx>"
   AZURE_LINUX_3_CORE_EFI_VHDX="<core-efi-3.0.vhdx>"
   AZURE_LINUX_4_CORE_EFI_VHDX="<core-efi-4.0.vhdx>"
   UBUNTU_2204_AZURE_CLOUD_VHD="<ubuntu-22.04-azure-cloud.vhd>"
   UBUNTU_2404_AZURE_CLOUD_VHD="<ubuntu-24.04-azure-cloud.vhd>"

   sudo go test -C ./toolkit/tools ./pkg/imagecustomizerlib -args \
     --base-image-core-efi-azl2 "$AZURE_LINUX_2_CORE_EFI_VHDX" \
     --base-image-core-efi-azl3 "$AZURE_LINUX_3_CORE_EFI_VHDX" \
     --base-image-core-efi-azl4 "$AZURE_LINUX_4_CORE_EFI_VHDX" \
     --base-image-azure-cloud-ubuntu2204 "$UBUNTU_2204_AZURE_CLOUD_VHD" \
     --base-image-azure-cloud-ubuntu2404 "$UBUNTU_2404_AZURE_CLOUD_VHD"
   ```

4. To update go dependencies (direct and indirect) to minor or patch versions,
   run `go get -u ./...` then `go mod tidy`.
