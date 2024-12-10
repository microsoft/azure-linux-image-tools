// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
)

const (
	SELinuxPolicyMinVersion0       = 2
	SELinuxPolicyMinVersion1       = 20240226
	SELinuxPolicyMinPackageRelease = 9
	SELinuxPolicyDistroName        = "azl"
	SELinuxPolicyMinDistroVersion  = 3
)

func verifySELinuxPolicyLiveOSISOSupport(packageInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{SELinuxPolicyMinVersion0, SELinuxPolicyMinVersion1},
		PackageRelease:           SELinuxPolicyMinPackageRelease,
		DistroName:               SELinuxPolicyDistroName,
		DistroVersion:            SELinuxPolicyMinDistroVersion,
	}
	packageName := "selinux-policy"
	err := verifyMinimumVersion(packageInfo, minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support SELinux with LiveOS ISOs.", packageName)
	}
	return nil
}
