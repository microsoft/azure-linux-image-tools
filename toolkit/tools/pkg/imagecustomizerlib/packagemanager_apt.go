// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

// aptPackageManager implements debPackageManagerHandler for APT.
type aptPackageManager struct{}

func newAptPackageManager() *aptPackageManager {
	return &aptPackageManager{}
}

func (pm *aptPackageManager) getPackageManagerBinary() string {
	return string(packageManagerAPT)
}

func (pm *aptPackageManager) getEnvironmentVariables() []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive",
		"DEBCONF_NONINTERACTIVE_SEEN=true",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
}

func (pm *aptPackageManager) isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	err := imageChroot.UnsafeRun(func() error {
		_, _, err := shell.Execute("dpkg-query", "-W", "-f='${Status}'", packageName)
		return err
	})
	if err != nil {
		return false
	}
	return true
}
