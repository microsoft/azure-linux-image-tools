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

// fedoraDistroHandler implements distroHandler for Fedora
type fedoraDistroHandler struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newFedoraDistroHandler(version string) *fedoraDistroHandler {
	return &fedoraDistroHandler{
		version:        version,
		packageManager: newDnfPackageManager(version),
	}
}

func (d *fedoraDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "42":
		return targetos.TargetOsFedora42
	default:
		panic("unsupported Fedora version: " + d.version)
	}
}

// managePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroHandler) managePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager,
	)
}

func (d *fedoraDistroHandler) isPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *fedoraDistroHandler) getAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *fedoraDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if d.isPackageInstalled(imageChroot, "grub2-efi-x64") || d.isPackageInstalled(imageChroot, "grub2-efi-aa64") {
		return BootloaderTypeGrub, nil
	}
	if d.isPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub2-efi-x64, grub2-efi-aa64, nor systemd-boot found")
}

func (d *fedoraDistroHandler) getGrubConfigFilePath(imageChroot safechroot.ChrootInterface) string {
	return filepath.Join(imageChroot.RootDir(), installutils.GrubCfgFile)
}
