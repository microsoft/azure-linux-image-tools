// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"fmt"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/envfile"
)

type TargetOs string

const (
	TargetOsAzureLinux2 TargetOs = "azl2"
	TargetOsAzureLinux3 TargetOs = "azl3"
	TargetOsFedora42    TargetOs = "fedora42"
)

func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	fields, err := envfile.ParseEnvFile(filepath.Join(rootfs, "etc/os-release"))
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/os-release file:\n%w", err)
	}

	distroId := fields["ID"]
	versionId := fields["VERSION_ID"]

	switch distroId {
	case "mariner":
		switch versionId {
		case "2.0":
			return TargetOsAzureLinux2, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for CBL-Mariner in /etc/os-release", versionId)
		}

	case "azurelinux":
		switch versionId {
		case "3.0":
			return TargetOsAzureLinux3, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Azure Linux in /etc/os-release", versionId)
		}

	case "fedora":
		switch versionId {
		case "42":
			return TargetOsFedora42, nil

		default:
			return "", fmt.Errorf("unknown VERSION_ID (%s) for Fedora in /etc/os-release", versionId)
		}

	default:
		return "", fmt.Errorf("unknown ID (%s) in /etc/os-release", distroId)
	}
}
