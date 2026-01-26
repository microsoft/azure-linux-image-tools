// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

// azureLinuxDistroHandler implements distroHandler for Azure Linux
type azureLinuxDistroHandler struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newAzureLinuxDistroHandler(version string) *azureLinuxDistroHandler {
	return &azureLinuxDistroHandler{
		version:        version,
		packageManager: newTdnfPackageManager(version),
	}
}

func (d *azureLinuxDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "2.0":
		return targetos.TargetOsAzureLinux2
	case "3.0":
		return targetos.TargetOsAzureLinux3
	default:
		panic("unsupported Azure Linux version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroHandler) managePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

// isPackageInstalled implements distroHandler.
func (d *azureLinuxDistroHandler) isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *azureLinuxDistroHandler) getAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *azureLinuxDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if d.isPackageInstalled(imageChroot, "grub2-efi-binary") || d.isPackageInstalled(imageChroot, "grub2-efi-binary-noprefix") {
		return BootloaderTypeGrub, nil
	}
	if d.isPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub2-efi-binary, grub2-efi-binary-noprefix, nor systemd-boot found")
}

func (d *azureLinuxDistroHandler) getGrubConfigFilePath(imageChroot safechroot.ChrootInterface) string {
	return filepath.Join(imageChroot.RootDir(), installutils.GrubCfgFile)
}
