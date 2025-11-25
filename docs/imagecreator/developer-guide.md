---
parent: Image Creator
nav_order: 5
---

# Developers guide

## Prerequisites

- Golang
- [Trivy](https://github.com/aquasecurity/trivy/releases/latest) for license scanning

For other package dependencies, see [Getting started guide](./quick-start/quick-start.md)

## Build Image Creator binary

Run:

```bash
sudo make -C ./toolkit go-imagecreator
```

After running the command, the Image Creator binary will be located at
`./toolkit/out/tools/imagecreator`.

## Run Image Creator specific tests

1. Download the test RPM files and the tools tar file to create new image:

   ```bash
   ./toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -d azurelinux -t 3.0 -s true
   ```

   For Fedora, download the test RPM files and tools tar file using:

   ```bash
   ./toolkit/tools/internal/testutils/testrpms/download-test-utils.sh -d fedora -t 42 -s true
   ```

2. Run the tests:

   ```bash
   sudo go test -C ./toolkit/tools ./pkg/imagecreatorlib -args \
     --run-create-image-tests true
   ```

3. To update go dependencies (direct and indirect) to minor or patch versions,
   run `go get -u ./...` then `go mod tidy`.
