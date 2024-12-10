// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

const (
	PxeDracutMinVersion        = 102
	PxeDracutMinPackageRelease = 7
	PxeDracutDistroName        = "azl"
	PxeDracutMinDistroVersion  = 3

	LiveISOSelinuxDracutMinVersion        = 102
	LiveISOSelinuxDracutMinPackageRelease = 8
	LiveISOSelinuxDracutDistroName        = "azl"
	LiveISOSelinuxDracutMinDistroVersion  = 3
)

func addDracutConfig(dracutConfigFile string, lines []string) error {
	if _, err := os.Stat(dracutConfigFile); os.IsNotExist(err) {
		err := file.WriteLines(lines, dracutConfigFile)
		if err != nil {
			return fmt.Errorf("failed to write to dracut config file (%s): %w", dracutConfigFile, err)
		}
	} else {
		return fmt.Errorf("dracut config file (%s) already exists", dracutConfigFile)
	}
	return nil
}

func addDracutModuleAndDriver(dracutModuleName string, dracutDriverName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), "etc", "dracut.conf.d", dracutModuleName+".conf")
	lines := []string{
		"add_dracutmodules+=\" " + dracutModuleName + " \"",
		"add_drivers+=\" " + dracutDriverName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}

func addDracutModule(dracutModuleName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), "etc", "dracut.conf.d", dracutModuleName+".conf")
	lines := []string{
		"add_dracutmodules+=\" " + dracutModuleName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}

func addDracutDriver(dracutDriverName string, imageChroot *safechroot.Chroot) error {
	dracutConfigFile := filepath.Join(imageChroot.RootDir(), "etc", "dracut.conf.d", dracutDriverName+".conf")
	lines := []string{
		"add_drivers+=\" " + dracutDriverName + " \"",
	}
	return addDracutConfig(dracutConfigFile, lines)
}

func verifyDracutPXESupport(packageVersionInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{PxeDracutMinVersion},
		PackageRelease:           PxeDracutMinPackageRelease,
		DistroName:               PxeDracutDistroName,
		DistroVersion:            PxeDracutMinDistroVersion,
	}
	packageName := "dracut"
	err := verifyMinimumVersion(packageVersionInfo, minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support PXE boot with LiveOS ISOs.", packageName)
	}
	return nil
}

func verifyDracutLiveISOSELinuxSupport(packageVersionInfo *PackageVersionInformation) error {
	minimumVersionPackageInfo := &PackageVersionInformation{
		PackageVersionComponents: []uint64{LiveISOSelinuxDracutMinVersion},
		PackageRelease:           LiveISOSelinuxDracutMinPackageRelease,
		DistroName:               LiveISOSelinuxDracutDistroName,
		DistroVersion:            LiveISOSelinuxDracutMinDistroVersion,
	}
	packageName := "dracut"
	err := verifyMinimumVersion(packageVersionInfo, minimumVersionPackageInfo)
	if err != nil {
		return fmt.Errorf("did not find the minimum (%s) required version to support SELinux with LiveOS ISOs.", packageName)
	}
	return nil
}
