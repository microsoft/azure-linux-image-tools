// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

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
