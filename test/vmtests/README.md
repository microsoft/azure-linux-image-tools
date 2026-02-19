# VMTests

A test suite that runs the containerized version of the image customizer tool and then
boots the customized images.

## How to run

Requirements:

- Python3
- QEMU/KVM
- libvirt
- A generated SSH private/public key pair.

Steps:

1. Set up Python venv:

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

## Output directory

Test outputs (logs, reports, and pytest log files) are saved to a timestamped directory
under `./out/`. For example: `./out/20260218.1430/`.

The `OUT_DIR` variable controls this path. By default, it is set to
`./out/<YYYYMMDD.HHMM>` based on the current date and time. The output directory
contains:

- `report.xml` - JUnit XML test report.
- `pytest.log` - Full pytest log file.
- `logs/` - VM and tool log files collected during the test run.

To override the output directory:

```bash
make test-imagecustomizer OUT_DIR=./out/my-run ...
```

## Debugging

If you want to keep the resources that the test creates around after the test has
finished running, then add:

```bash
KEEP_ENVIRONMENT=y
```

to the `make` call.

## Filtering tests

To run only tests matching a specific expression, set the `TEST_FILTER` variable. This
is passed to pytest's `-k` option:

```bash
make test-imagecustomizer TEST_FILTER="test_min_change_efi_azl2"
```

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

For packages required for auxiliary dev tasks (e.g. linting), add them to
[requirements/dev.txt](./requirements/dev.txt).

Since there is no lock file, all packages should be specified using an exact version.

For example:

```ini
pytest == 8.3.3
```

This ensures that everyone is using consistent versions of the packages.
