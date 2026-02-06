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

// ubuntuDistroHandler implements distroHandler for Ubuntu
type ubuntuDistroHandler struct {
	version string
}

func newUbuntuDistroHandler(version string) *ubuntuDistroHandler {
	return &ubuntuDistroHandler{
		version: version,
	}
}

func (d *ubuntuDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "22.04":
		return targetos.TargetOsUbuntu2204
	case "24.04":
		return targetos.TargetOsUbuntu2404
	default:
		panic("unsupported Ubuntu version: " + d.version)
	}
}

// ManagePackages handles the complete package management workflow for Ubuntu
func (d *ubuntuDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	if config != nil && (len(config.Packages.Install) > 0 || len(config.Packages.Remove) > 0 || len(config.Packages.Update) > 0) {
		return fmt.Errorf("package management customizations are not yet supported for Ubuntu")
	}

	return nil
}

// IsPackageInstalled checks if a package is installed using dpkg
func (d *ubuntuDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return isPackageInstalledDpkg(imageChroot, packageName)
}

func (d *ubuntuDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootDpkg(imageChroot)
}

func (d *ubuntuDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if d.IsPackageInstalled(imageChroot, "grub-efi-amd64") || d.IsPackageInstalled(imageChroot, "grub-efi-arm64") || d.IsPackageInstalled(imageChroot, "grub-efi") {
		return BootloaderTypeGrub, nil
	}
	if d.IsPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub-efi-amd64, grub-efi-arm64, nor systemd-boot found")
}

func (d *ubuntuDistroHandler) GetGrubConfigFilePath(imageChroot safechroot.ChrootInterface) string {
	return filepath.Join(imageChroot.RootDir(), installutils.UbuntuGrubCfgFile)
}

func (d *ubuntuDistroHandler) SupportsSELinux() bool {
	return false
}
