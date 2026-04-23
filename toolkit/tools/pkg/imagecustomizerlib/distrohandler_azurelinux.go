// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

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
	var packageManager rpmPackageManagerHandler
	if version == "4.0" {
		packageManager = newDnfPackageManager(version)
	} else {
		packageManager = newTdnfPackageManager(version)
	}

	return &azureLinuxDistroHandler{
		version:        version,
		packageManager: packageManager,
	}
}

func (d *azureLinuxDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "2.0":
		return targetos.TargetOsAzureLinux2
	case "3.0":
		return targetos.TargetOsAzureLinux3
	case "4.0":
		return targetos.TargetOsAzureLinux4
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
	grubPackages := []string{"grub2-efi-binary", "grub2-efi-binary-noprefix"}
	if d.version == "4.0" {
		grubPackages = []string{"grub2-efi-x64", "grub2-efi-aa64"}
	}
	for _, pkg := range grubPackages {
		if d.IsPackageInstalled(imageChroot, pkg) {
			return BootloaderTypeGrub, nil
		}
	}
	systemdBootPackage := "systemd-boot"
	if d.version == "4.0" {
		systemdBootPackage = "systemd-boot-unsigned"
	}
	if d.IsPackageInstalled(imageChroot, systemdBootPackage) {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf("unknown bootloader: none of %v or %s found", grubPackages, systemdBootPackage)
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
		currentSELinuxMode, forceGrubMkconfig)
}

func (d *azureLinuxDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	if d.version == "4.0" {
		// Azure Linux 4.0 uses BLS (Boot Loader Specification).
		return readKernelCmdlinesFromBLSEntries(bootDir)
	}

	// Azure Linux 2.0/3.0 uses grub.cfg with inline linux commands.
	return readKernelCmdlinesFromGrubCfg(bootDir, FedoraGrubCfgPath)
}

func (d *azureLinuxDistroHandler) ReadKernelCmdlines(bootDir string) (map[string]string, error) {
	kernelToArgs, err := d.ReadGrubConfigLinuxArgs(bootDir)
	if err != nil {
		return nil, err
	}

	return grubKernelArgsToStringMap(kernelToArgs), nil
}

func (d *azureLinuxDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	if d.version == "4.0" {
		return readNonRecoveryKernelCmdlinesFromBLS(bootDir, argNames)
	}

	grubCfgPath := filepath.Join(bootDir, FedoraGrubCfgPath)
	return readNonRecoveryKernelCmdlinesFromGrubCfg(grubCfgPath, argNames)
}
