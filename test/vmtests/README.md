# VM Tests

## How to run

Requirements:

- Python3

Steps:

1. Setup Python venv:

   ```sh
   make create-venv
   ```

2. Run:

   ```bash
   make run
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
