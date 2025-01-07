// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package targetos

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
)

type TargetOs string

const (
	TargetOsAzureLinux2 TargetOs = "azl2"
	TargetOsAzureLinux3 TargetOs = "azl3"
)

func GetInstalledTargetOs(rootfs string) (TargetOs, error) {
	osReleaseLines, err := file.ReadLines(filepath.Join(rootfs, "etc/os-release"))
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/os-release file:\n%w", err)
	}

	fields := make(map[string]string)

	for _, line := range osReleaseLines {

		split := strings.SplitN(line, "=", 2)
		if len(split) < 2 {
			continue
		}
		name := split[0]
		value := split[1]

		value = strings.TrimPrefix(value, "\"")
		value = strings.TrimSuffix(value, "\"")

		fields[name] = value
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

	default:
		return "", fmt.Errorf("unknown ID (%s) in /etc/os-release", distroId)
	}
}
