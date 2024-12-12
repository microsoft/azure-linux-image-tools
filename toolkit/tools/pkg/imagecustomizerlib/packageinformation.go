// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

type PackageVersionInformation struct {
	PackageVersionComponents []uint64 `yaml:"PackageVersionComponents"`
	PackageRelease           uint32   `yaml:"PackageRelease"`
	DistroName               string   `yaml:"DistroName"`
	DistroVersion            uint32   `yaml:"DistroVersion"`
}

func (pi *PackageVersionInformation) getVersionString() string {
	var version strings.Builder
	for i, versionComponent := range pi.PackageVersionComponents {
		if i != 0 {
			version.WriteString(".")
		}
		version.WriteString(strconv.FormatUint(versionComponent, 10))
	}
	return version.String()
}

func (pi *PackageVersionInformation) getFullVersionString() string {
	// yy.yy.yy-zz.azl3
	return fmt.Sprintf("%s-%d.%s%d", pi.getVersionString(), pi.PackageRelease, pi.DistroName, pi.DistroVersion)
}

func (pi *PackageVersionInformation) verifyMinimumVersion(minimumVersionInfo *PackageVersionInformation) error {
	if minimumVersionInfo == nil {
		panic("input package information undefined")
	}

	minimumVersion := minimumVersionInfo.getFullVersionString()
	currentVersion := pi.getFullVersionString()

	if pi.DistroName != minimumVersionInfo.DistroName {
		return fmt.Errorf("did not find required distro (%s) - found (%s)", minimumVersion, currentVersion)
	}

	if pi.DistroVersion < minimumVersionInfo.DistroVersion {
		return fmt.Errorf("did not find required distro version (%s) (or newer) - found (%s)", minimumVersion, currentVersion)
	}

	// Note that, theoretically, a newer distro version could still have an older package version.
	// So, it is not sufficient to check that packageInfo.DistroVersion > MinDistroVersion.
	// We need to check the package version number.

	if len(pi.PackageVersionComponents) != len(minimumVersionInfo.PackageVersionComponents) {
		return fmt.Errorf("unexpected number of version components (%s) - found (%s)", minimumVersion, currentVersion)
	}

	for i, versionComponent := range pi.PackageVersionComponents {
		if versionComponent < minimumVersionInfo.PackageVersionComponents[i] {
			return fmt.Errorf("did not find required package version (%s) (or newer) - found (%s)", minimumVersion, currentVersion)
		} else if versionComponent > minimumVersionInfo.PackageVersionComponents[i] {
			return nil
		}
	}

	if pi.PackageRelease < minimumVersionInfo.PackageRelease {
		return fmt.Errorf("did not find required package release version (%s) (or newer) - found (%s)", minimumVersion, currentVersion)
	}

	return nil
}

func parseReleaseString(releaseInfo string) (packageRelease uint32, distroName string, distroVersion uint32, err error) {
	pattern := `([0-9]+)\.([a-zA-Z]+)([0-9]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(releaseInfo)

	if matches == nil {
		return 0, "", 0, fmt.Errorf("failed to parse package release information (%s)\n%w", releaseInfo, err)
	}

	// package release
	packageReleaseString := matches[1]
	packageReleaseUint64, err := strconv.ParseUint(packageReleaseString, 10 /*base*/, 32 /*size*/)
	if err != nil {
		return 0, "", 0, fmt.Errorf("failed to parse package release version (%s) into an unsigned integer:\n%w", packageReleaseString, err)
	}
	packageRelease = uint32(packageReleaseUint64)

	// distro name
	distroName = matches[2]

	// distro version
	distroVersionString := matches[3]
	distroVersionUint64, err := strconv.ParseUint(distroVersionString, 10 /*base*/, 32 /*size*/)
	if err != nil {
		return 0, "", 0, fmt.Errorf("failed to parse distro version (%s) into an unsigned integer:\n%w", distroVersionString, err)
	}
	distroVersion = uint32(distroVersionUint64)

	return packageRelease, distroName, distroVersion, nil
}

func parseVersionString(version string) ([]uint64, error) {
	// Regular expression to capture version components
	// Expected patterns are: "number(.number)*"
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))*$`)

	// Match the version string against the regex
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	// Extract all captured groups
	var versionComponents []uint64
	for _, match := range matches[1:] {
		// Given a pattern is meant to match zero or more time:
		// - when it does not match (i.e. matches 0 times), golang still adds
		//   an empty match.
		// So, for versions like "102", the second group in the regex will
		// not match (i.e. no ".xyz"), and an empty match will be inserted.
		if match == "" {
			continue
		}
		versionComponent, err := strconv.ParseUint(match, 10 /*base*/, 64 /*size*/)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package version component (%s) into an unsigned integer:\n%w", match, err)
		}
		versionComponents = append(versionComponents, versionComponent)
	}

	return versionComponents, nil
}

func getPackageInformation(imageChroot *safechroot.Chroot, packageName string) (info *PackageVersionInformation, err error) {
	var packageInfo string
	err = imageChroot.UnsafeRun(func() error {
		packageInfo, _, err = shell.Execute("tdnf", "info", packageName, "--repo", "@system")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query (%s) package information:\n%w", packageName, err)
	}

	// Regular expressions to match Version and Release
	versionRegex := regexp.MustCompile(`(?m)^Version\s+:\s+(\S+)`)
	versionMatch := versionRegex.FindStringSubmatch(packageInfo)
	var packageVersion string
	if len(versionMatch) != 2 {
		return nil, fmt.Errorf("failed to extract version information from the (%s) package information (\n%s\n):\n%w", packageName, packageInfo, err)
	}
	packageVersion = versionMatch[1]

	versionComponents, err := parseVersionString(packageVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the (%s) package version information (%s):\n%w", packageName, packageVersion, err)
	}

	// Extract Release
	releaseRegex := regexp.MustCompile(`(?m)^Release\s+:\s+(\S+)`)
	releaseMatch := releaseRegex.FindStringSubmatch(packageInfo)
	var releaseInfo string
	if len(releaseMatch) != 2 {
		return nil, fmt.Errorf("failed to extract release information from the (%s) package information (\n%s\n):\n%w", packageName, packageInfo, err)
	}
	releaseInfo = releaseMatch[1]

	packageRelease, distroName, distroVersion, err := parseReleaseString(releaseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse release information for package (%s)\n%w", packageName, err)
	}

	// Set return values
	info = &PackageVersionInformation{
		PackageVersionComponents: versionComponents,
		PackageRelease:           packageRelease,
		DistroName:               distroName,
		DistroVersion:            distroVersion,
	}

	return info, nil
}
