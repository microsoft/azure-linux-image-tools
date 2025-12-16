# VM Tests

A test suite that runs the containerized version of the image customizer tool and then
boots the customized images.

## How to run

Requirements:

- Python3
- QEMU/KVM
- libvirt
- A generated SSH private/public key pair.

Steps:

1. Setup Python venv:

   ```sh
   make create-venv
   ```

2. Download a copy of the Azure Linux 2.0 core-efi VHDX image file.

3. Run:

   ```bash
   CORE_EFI_AZL2="<core-efi-vhdx>"
   make run CORE_EFI_AZL2="$CORE_EFI_AZL2"
   ```

   Where:

   - `<core-efi-vhdx>` is the path of the VHDX file downloaded in Step 2.

   Note: By default, the `${HOME}/.ssh/id_ed25519` SSH private key is used. If you want
   to use a different private key, then set the `SSH_PRIVATE_KEY_FILE` variable when
   calling `make`.

## Debugging

If you want to keep the resources that the test creates around after the test has
finished running, then add:

```bash
KEEP_ENVIRONMENT=y
```

to the `make` call.

## Linting, mypy, and other code checks

This project uses Black for automatic code formatting, isort for sorting imports, mypy
for type checking, and flake8 for code format checking.

All of these tools can be run together by running:

```bash
make fix check
```

## Adding Python packages

For packages required to run the test suite, add them to
[requirements.txt](./requirements.txt).

For packages required for auxiliary dev tasks (e.g., linting), add them to
[requirements/dev.txt](./requirements/dev.txt).

Since there is no lock file, all packages should be specified using an exact version.

For example:

```ini
pytest == 8.3.3
```

This ensures that everyone is using consistent versions of the packages.
