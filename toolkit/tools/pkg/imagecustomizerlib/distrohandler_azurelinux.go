// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
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

func (d *azureLinuxDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	return nil
}

// ManagePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

// IsPackageInstalled implements DistroHandler.
func (d *azureLinuxDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *azureLinuxDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *azureLinuxDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	if d.IsPackageInstalled(imageChroot, "grub2-efi-binary") || d.IsPackageInstalled(imageChroot, "grub2-efi-binary-noprefix") {
		return BootloaderTypeGrub, nil
	}
	if d.IsPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: neither grub2-efi-binary, grub2-efi-binary-noprefix, nor systemd-boot found")
}

func (d *azureLinuxDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *azureLinuxDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *azureLinuxDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinuxDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinuxDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs file")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	var err error
	if d.version == "2.0" {
		// The 'mkinitrd' command was removed in Azure Linux 3.0 in favor of using 'dracut' directly.
		err = shell.NewExecBuilder("mkinitrd").
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Chroot(imageChroot.ChrootDir()).
			Execute()
	} else {
		err = shell.NewExecBuilder("dracut", "--force", "--regenerate-all").
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Chroot(imageChroot.ChrootDir()).
			Execute()
	}
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs:\n%w", err)
	}

	return nil
}

func (d *azureLinuxDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	// Azure Linux 3.0+ always uses grub-mkconfig.
	// The legacy grub config detection logic is only relevant for Azure Linux 2.0.
	// And for new images, always use grub-mkconfig.
	forceGrubMkconfig := newImage || d.version != "2.0"

	return configureDiskBootLoader(imageConnection, rootMountIdType, bootType, selinuxConfig, kernelCommandLine,
		currentSELinuxMode, forceGrubMkconfig, d)
}
