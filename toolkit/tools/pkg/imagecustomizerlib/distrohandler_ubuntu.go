// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
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
	if len(rpmsSources) > 0 {
		return fmt.Errorf("RPM sources are not supported for Ubuntu images:\n%w", ErrUnsupportedUbuntuFeature)
	}

	// UseBaseImageRpmRepos defaults to true and is only false when the user explicitly
	// passes --disable-base-image-rpm-repos. Ubuntu does not use RPM repos, so disabling
	// them is not meaningful and likely indicates a configuration mistake.
	if !useBaseImageRpmRepos {
		return fmt.Errorf("Disabling base image RPM repositories is not supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	packages := config.Packages

	if len(packages.Remove) > 0 || len(packages.RemoveLists) > 0 {
		return fmt.Errorf("package remove is not yet supported for Ubuntu images:\n%w", ErrUnsupportedUbuntuFeature)
	}

	if len(packages.Update) > 0 || len(packages.UpdateLists) > 0 {
		return fmt.Errorf("package update is not yet supported for Ubuntu images:\n%w", ErrUnsupportedUbuntuFeature)
	}

	if packages.UpdateExistingPackages {
		return fmt.Errorf("updateExistingPackages is not yet supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	if packages.SnapshotTime != "" {
		return fmt.Errorf("package snapshotTime is not yet supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	return managePackagesDeb(ctx, config, imageChroot)
}

// IsPackageInstalled checks if a package is installed using dpkg-query.
func (d *ubuntuDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return isPackageInstalledDeb(imageChroot, packageName)
}

func (d *ubuntuDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootDeb(imageChroot)
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

func (d *ubuntuDistroHandler) ReadGrubConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	grubCfgFilePath := filepath.Join(imageChroot.RootDir(), installutils.UbuntuGrubCfgFile)
	grubCfgContent, err := file.Read(grubCfgFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read grub config file (%s):\n%w", installutils.UbuntuGrubCfgFile, err)
	}
	return grubCfgContent, nil
}

func (d *ubuntuDistroHandler) WriteGrubConfigFile(grubCfgContent string,
	imageChroot safechroot.ChrootInterface,
) error {
	grubCfgFilePath := filepath.Join(imageChroot.RootDir(), installutils.UbuntuGrubCfgFile)
	err := file.Write(grubCfgContent, grubCfgFilePath)
	if err != nil {
		return fmt.Errorf("failed to write grub config file (%s):\n%w", installutils.UbuntuGrubCfgFile, err)
	}
	return nil
}

func (d *ubuntuDistroHandler) SELinuxSupported() bool {
	return false
}

func (d *ubuntuDistroHandler) InstallBootloader(imageChroot *safechroot.Chroot,
	bootType string, bootUUID string, bootPrefix string, diskDevPath string,
) error {
	return fmt.Errorf("bootloader installation is not yet implemented for Ubuntu images:\n%w",
		ErrUnsupportedUbuntuFeature)
}

func (d *ubuntuDistroHandler) InstallGrubDefaults(imageChroot *safechroot.Chroot,
	rootDevice string, bootUUID string, bootPrefix string,
	kernelCommandLine configuration.KernelCommandLine,
	isBootPartitionSeparate bool, grubMkconfigEnabled bool,
) error {
	return fmt.Errorf("grub defaults installation is not yet implemented for Ubuntu images:\n%w",
		ErrUnsupportedUbuntuFeature)
}

func (d *ubuntuDistroHandler) CallGrubMkconfig(imageChroot safechroot.ChrootInterface) error {
	return fmt.Errorf("grub-mkconfig is not yet implemented for Ubuntu images:\n%w",
		ErrUnsupportedUbuntuFeature)
}
