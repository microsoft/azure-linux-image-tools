// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
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
	err := verifyMinimumVersion(packageName, packageInfo, minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support SELinux with LiveOS ISOs.", packageName)
	}
	return nil
}

func isSELinuxEnabled(rootFolder string) (bool, error) {
	chroot := safechroot.NewChroot(rootFolder, true /*isExistingDir*/)
	if chroot == nil {
		return false, fmt.Errorf("failed to create a new chroot object for %s.", rootFolder)
	}
	defer chroot.Close(true /*leaveOnDisk*/)

	err := chroot.Initialize("", nil, nil, true /*includeDefaultMounts*/)
	if err != nil {
		return false, fmt.Errorf("failed to initialize chroot object for %s:\n%w", rootFolder, err)
	}

	bootCustomizer, err := NewBootCustomizer(chroot)
	if err != nil {
		return false, err
	}

	currentSELinuxMode, err := bootCustomizer.GetSELinuxMode(chroot)
	if err != nil {
		return false, fmt.Errorf("failed to get current SELinux mode:\n%w", err)
	}

	return currentSELinuxMode != imagecustomizerapi.SELinuxModeDisabled, nil
}
