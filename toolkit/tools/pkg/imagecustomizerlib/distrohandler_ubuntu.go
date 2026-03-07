// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

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

func (d *ubuntuDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	switch d.version {
	case "22.04":
		if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureUbuntu2204) {
			return ErrUbuntu2204PreviewFeatureRequired
		}
	case "24.04":
		if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureUbuntu2404) {
			return ErrUbuntu2404PreviewFeatureRequired
		}

	default:
		panic("unsupported Ubuntu version: " + d.version)
	}

	// Check if Ubuntu is being used with bootloader hard-reset.
	// Ubuntu bootloader config logic is not yet fully implemented.
	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return ErrUbuntuBootLoaderHardReset
	}

	return nil
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

	if config.Packages.SnapshotTime != "" {
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

func (d *ubuntuDistroHandler) GetGrubConfigFilePath(imageChroot safechroot.ChrootInterface) string {
	return filepath.Join(imageChroot.RootDir(), installutils.UbuntuGrubCfgFile)
}

func (d *ubuntuDistroHandler) SELinuxSupported() bool {
	return false
}
