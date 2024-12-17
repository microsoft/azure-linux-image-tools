// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
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
	for _, line := range osReleaseLines {
		strings.SplitN(line, "=", 1)
	}
}
