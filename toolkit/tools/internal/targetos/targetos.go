// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
)

type TargetOs string

const (
	TargetOsAzureLinux2 TargetOs = "azl2"
	TargetOsAzureLinux3 TargetOs = "azl3"
	TargetOsAzureContainerLinux3 TargetOs = "acl3"
	TargetOsFedora42    TargetOs = "fedora42"
	TargetOsUbuntu2204  TargetOs = "ubuntu2204"
	TargetOsUbuntu2404  TargetOs = "ubuntu2404"
)

func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	// Try /etc/os-release first, then fall back to /usr/lib/os-release.
	fields, err := envfile.ParseEnvFile(filepath.Join(rootfs, "etc/os-release"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("failed to read /etc/os-release:\n%w", err)
		}
		fields, err = envfile.ParseEnvFile(filepath.Join(rootfs, "usr/lib/os-release"))
		if err != nil {
			return "", fmt.Errorf("failed to read os-release (tried /etc/os-release and /usr/lib/os-release):\n%w", err)
		}
	}

	distroId := fields["ID"]
	versionId := fields["VERSION_ID"]

	switch distroId {
	case "mariner":
		switch versionId {
		case "2.0":
			return TargetOsAzureLinux2, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for CBL-Mariner in os-release", versionId)
		}

	case "azurelinux":
		variantId := fields["VARIANT_ID"]

		switch variantId {
		case "azurecontainerlinux":
			// ACL uses VERSION_ID like "3.0.YYYYMMDD" (e.g. "3.0.20260421").
			// Accept any version that starts with "3.0.".
			if !strings.HasPrefix(versionId, "3.0.") {
				return "", fmt.Errorf("unknown VERSION_ID (%s) for Azure Container Linux in os-release", versionId)
			}
			return TargetOsAzureContainerLinux3, nil

		default:
			// Standard Azure Linux (or unknown variant — treat as standard).
			switch versionId {
			case "3.0":
				return TargetOsAzureLinux3, nil

			default:
				return "", fmt.Errorf("unknown VERSION_ID (%s) for Azure Linux in os-release", versionId)
			}
		}

	case "fedora":
		switch versionId {
		case "42":
			return TargetOsFedora42, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Fedora in os-release", versionId)
		}

	case "ubuntu":
		switch versionId {
		case "22.04":
			return TargetOsUbuntu2204, nil

		case "24.04":
			return TargetOsUbuntu2404, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Ubuntu in os-release", versionId)
		}

	default:
		return "", fmt.Errorf("unknown ID (%s) in os-release", distroId)
	}
}
