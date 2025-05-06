// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
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

type BootFilesArchConfig struct {
	bootBinary                  string
	grubBinary                  string
	grubNoPrefixBinary          string
	systemdBootBinary           string
	osEspBootBinaryPath         string
	osEspGrubBinaryPath         string
	osEspGrubNoPrefixBinaryPath string
	isoBootBinaryPath           string
	isoGrubBinaryPath           string
}

var (
	bootloaderFilesConfig = map[string]BootFilesArchConfig{
		"amd64": {
			bootBinary:                  bootx64Binary,
			grubBinary:                  grubx64Binary,
			grubNoPrefixBinary:          grubx64NoPrefixBinary,
			systemdBootBinary:           systemdBootx64Binary,
			osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootx64Binary,
			osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubx64Binary,
			osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubx64NoPrefixBinary,
			isoBootBinaryPath:           isoBootloaderDir + "/" + bootx64Binary,
			isoGrubBinaryPath:           isoBootloaderDir + "/" + grubx64Binary,
		},
		"arm64": {
			bootBinary:                  bootAA64Binary,
			grubBinary:                  grubAA64Binary,
			grubNoPrefixBinary:          grubAA64NoPrefixBinary,
			systemdBootBinary:           systemdBootAA64Binary,
			osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootAA64Binary,
			osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubAA64Binary,
			osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubAA64NoPrefixBinary,
			isoBootBinaryPath:           isoBootloaderDir + "/" + bootAA64Binary,
			isoGrubBinaryPath:           isoBootloaderDir + "/" + grubAA64Binary,
		},
	}
)

func getBootArchConfig() (string, BootFilesArchConfig, error) {
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return "", BootFilesArchConfig{}, fmt.Errorf("unsupported architecture: %s", arch)
	}
	return arch, bootloaderFilesConfig[arch], nil
}

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

func extractIsoImageContents(buildDir string, isoImageFile string, isoExpansionFolder string) (err error) {
	mountDir, err := os.MkdirTemp(buildDir, "tmp-iso-mount-")
	if err != nil {
		return fmt.Errorf("failed to create temporary mount folder for iso:\n%w", err)
	}
	defer os.RemoveAll(mountDir)

	isoImageLoopDevice, err := safeloopback.NewLoopback(isoImageFile)
	if err != nil {
		return fmt.Errorf("failed to create loop device for (%s):\n%w", isoImageFile, err)
	}
	defer isoImageLoopDevice.Close()

	isoImageMount, err := safemount.NewMount(isoImageLoopDevice.DevicePath(), mountDir,
		"iso9660" /*fstype*/, unix.MS_RDONLY /*flags*/, "" /*data*/, false /*makeAndDelete*/)
	if err != nil {
		return err
	}
	defer isoImageMount.Close()

	err = os.MkdirAll(isoExpansionFolder, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder %s:\n%w", isoExpansionFolder, err)
	}

	err = copyPartitionFiles(mountDir+"/.", isoExpansionFolder)
	if err != nil {
		return fmt.Errorf("failed to copy iso image contents to a writeable folder (%s):\n%w", isoExpansionFolder, err)
	}

	err = isoImageMount.CleanClose()
	if err != nil {
		return err
	}

	err = isoImageLoopDevice.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
