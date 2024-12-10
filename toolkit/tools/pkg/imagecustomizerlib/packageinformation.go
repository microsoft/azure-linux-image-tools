// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"strconv"
)

type PackageVersionInformation struct {
	PackageVersionComponents []uint64 `yaml:"PackageVersionComponents"`
	PackageRelease           uint32   `yaml:"PackageRelease"`
	DistroName               string   `yaml:"DistroName"`
	DistroVersion            uint32   `yaml:"DistroVersion"`
}

func (pi *PackageVersionInformation) getVersionString() (version string, err error) {
	if len(pi.PackageVersionComponents) == 0 {
		return "", fmt.Errorf("no version defined")
	}

	for i, versionComponent := range pi.PackageVersionComponents {
		if i != 0 {
			version += "."
		}
		version += strconv.FormatUint(versionComponent, 10)
	}
	return version, nil
}

func verifyMinimumVersion(packageInfo *PackageVersionInformation, minimumVersionInfo *PackageVersionInformation) error {
	if packageInfo == nil {
		return fmt.Errorf("no package information provided")
	}

	if packageInfo.DistroName != minimumVersionInfo.DistroName {
		return fmt.Errorf("did not find required Azure Linux distro (%s) - found (%s)", minimumVersionInfo.DistroName, packageInfo.DistroName)
	}

	if packageInfo.DistroVersion < minimumVersionInfo.DistroVersion {
		return fmt.Errorf("did not find required Azure Linux distro version (%d) - found (%d)", minimumVersionInfo.DistroVersion, packageInfo.DistroVersion)
	}

	// Note that, theoretically, a newer distro version could still have an older package version.
	// So, it is not sufficient to check that packageInfo.DistroVersion > MinDistroVersion.
	// We need to check the package version number.
	minimumVersion := fmt.Sprintf("%d-%d.%s%d", minimumVersionInfo.PackageVersionComponents[0], minimumVersionInfo.PackageRelease,
		minimumVersionInfo.DistroName, minimumVersionInfo.DistroVersion)

	if len(minimumVersionInfo.PackageVersionComponents) != len(packageInfo.PackageVersionComponents) {
		return fmt.Errorf("unexpected number of version components (%d) - found (%d)",
			len(minimumVersionInfo.PackageVersionComponents), len(packageInfo.PackageVersionComponents))
	}

	currentVersion := fmt.Sprintf("%d-%d.%s%d", packageInfo.PackageVersionComponents[0], packageInfo.PackageRelease, packageInfo.DistroName, packageInfo.DistroVersion)

	for i, versionComponent := range packageInfo.PackageVersionComponents {
		if versionComponent < minimumVersionInfo.PackageVersionComponents[i] {
			return fmt.Errorf("did not find required package version (%s) (or newer) - found (%s)", minimumVersion, currentVersion)
		} else if versionComponent > minimumVersionInfo.PackageVersionComponents[i] {
			return nil
		}
	}

	if packageInfo.PackageRelease < minimumVersionInfo.PackageRelease {
		return fmt.Errorf("did not find required package version (%s) (or newer) - found (%s)",
			minimumVersion, currentVersion)
	}

	return nil
}
