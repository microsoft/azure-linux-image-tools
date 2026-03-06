// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
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

// ManagePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager,
	)
}

func (d *fedoraDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *fedoraDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *fedoraDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if d.IsPackageInstalled(imageChroot, "grub2-efi-x64") || d.IsPackageInstalled(imageChroot, "grub2-efi-aa64") {
		return BootloaderTypeGrub, nil
	}
	if d.IsPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub2-efi-x64, grub2-efi-aa64, nor systemd-boot found")
}

func (d *fedoraDistroHandler) ReadGrubConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	grubCfgFilePath := filepath.Join(imageChroot.RootDir(), installutils.GrubCfgFile)
	grubCfgContent, err := file.Read(grubCfgFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read grub config file (%s):\n%w", installutils.GrubCfgFile, err)
	}
	return grubCfgContent, nil
}

func (d *fedoraDistroHandler) WriteGrubConfigFile(grubCfgContent string,
	imageChroot safechroot.ChrootInterface,
) error {
	grubCfgFilePath := filepath.Join(imageChroot.RootDir(), installutils.GrubCfgFile)
	err := file.Write(grubCfgContent, grubCfgFilePath)
	if err != nil {
		return fmt.Errorf("failed to write grub config file (%s):\n%w", installutils.GrubCfgFile, err)
	}
	return nil
}

func (d *fedoraDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *fedoraDistroHandler) InstallBootloader(imageChroot *safechroot.Chroot,
	bootType string, bootUUID string, bootPrefix string, diskDevPath string,
) error {
	return installutils.InstallBootloader(imageChroot, false, bootType, bootUUID, bootPrefix, diskDevPath)
}

func (d *fedoraDistroHandler) InstallGrubDefaults(imageChroot *safechroot.Chroot,
	rootDevice string, bootUUID string, bootPrefix string,
	kernelCommandLine configuration.KernelCommandLine,
	isBootPartitionSeparate bool, grubMkconfigEnabled bool,
) error {
	err := installutils.InstallGrubDefaults(imageChroot.RootDir(), rootDevice, bootUUID, bootPrefix,
		diskutils.EncryptedRootDevice{}, kernelCommandLine, isBootPartitionSeparate, !grubMkconfigEnabled)
	if err != nil {
		return err
	}

	err = installutils.InstallGrubEnv(imageChroot.RootDir())
	if err != nil {
		return err
	}

	return nil
}

func (d *fedoraDistroHandler) CallGrubMkconfig(imageChroot safechroot.ChrootInterface) error {
	return installutils.CallGrubMkconfig(imageChroot)
}
