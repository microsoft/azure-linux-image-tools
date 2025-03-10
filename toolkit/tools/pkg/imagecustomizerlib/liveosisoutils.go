// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
)

const (
	// Minimum dracut version required to enable PXE booting.
	LiveOsPxeDracutMinVersion        = 102
	LiveOsPxeDracutMinPackageRelease = 7
	LiveOsPxeDracutDistroName        = "azl"
	LiveOsPxeDracutMinDistroVersion  = 3

	// Minumum dracut version required to enable SELinux.
	LiveOsSelinuxDracutMinVersion        = 102
	LiveOsSelinuxDracutMinPackageRelease = 11
	LiveOsSelinuxDracutDistroName        = "azl"
	LiveOsSelinuxDracutMinDistroVersion  = 3

	// Minimum selinux-poicy version required to enable SELinux.
	LiveOsSelinuxPolicyMinVersion0       = 2
	LiveOsSelinuxPolicyMinVersion1       = 20240226
	LiveOsSelinuxPolicyMinPackageRelease = 9
	LiveOsSelinuxPolicyDistroName        = "azl"
	LiveOsSelinuxPolicyMinDistroVersion  = 3
)

// verifies that the dracut package supports PXE booting for LiveOS images.
func verifyDracutPXESupport(dracutVersionInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{LiveOsPxeDracutMinVersion},
		PackageRelease:           LiveOsPxeDracutMinPackageRelease,
		DistroName:               LiveOsPxeDracutDistroName,
		DistroVersion:            LiveOsPxeDracutMinDistroVersion,
	}
	packageName := "dracut"
	err := dracutVersionInfo.verifyMinimumVersion(minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support PXE boot with LiveOS ISOs:\n%w", packageName, err)
	}
	return nil
}

// verifies that the dracut package supports enabling SELinux for LiveOS images.
func verifyDracutLiveOsSELinuxSupport(dracutVersionInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{LiveOsSelinuxDracutMinVersion},
		PackageRelease:           LiveOsSelinuxDracutMinPackageRelease,
		DistroName:               LiveOsSelinuxDracutDistroName,
		DistroVersion:            LiveOsSelinuxDracutMinDistroVersion,
	}
	packageName := "dracut"
	err := dracutVersionInfo.verifyMinimumVersion(minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support SELinux with LiveOS ISOs:\n%w", packageName, err)
	}
	return nil
}

// verifies that the selinux-policy supports LiveOS images.
func verifySelinuxPolicyLiveOsSupport(selinuxPolicyVersionInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{LiveOsSelinuxPolicyMinVersion0, LiveOsSelinuxPolicyMinVersion1},
		PackageRelease:           LiveOsSelinuxPolicyMinPackageRelease,
		DistroName:               LiveOsSelinuxPolicyDistroName,
		DistroVersion:            LiveOsSelinuxPolicyMinDistroVersion,
	}
	packageName := "selinux-policy"
	err := selinuxPolicyVersionInfo.verifyMinimumVersion(minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support SELinux with LiveOS ISOs:\n%w", packageName, err)
	}
	return nil
}

// verifies that SELinux is can work for LiveOS images.
func verifyNoLiveOsSelinuxBlockers(dracutVersionInfo *PackageVersionInformation, selinuxPolicyVersionInfo *PackageVersionInformation) error {
	if dracutVersionInfo != nil {
		err := verifyDracutLiveOsSELinuxSupport(dracutVersionInfo)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("dracut package information is missing")
	}

	// selinuxPolicyVersionInfo is nil when selinux-policy is not installed.
	// If selinux is enabled, and selinux-policy is not installed, it means that
	// the user has a policy installed through a package unknown to us.
	// We will not report an error in such cases.
	if selinuxPolicyVersionInfo != nil {
		err := verifySelinuxPolicyLiveOsSupport(selinuxPolicyVersionInfo)
		if err != nil {
			return err
		}
	}

	return nil
}
