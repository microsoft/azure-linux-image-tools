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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
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

func (d *ubuntuDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *ubuntuDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// Reading Ubuntu's grub.cfg stub is not supported, so for now just use Azure Linux 3.0's values.
	return readBootPartitionUuidFromGrubCfg(filepath.Join(espMountDir, espGrubCfgPathAzl3), bootPartitionRegexAzl3)
}

func (d *ubuntuDistroHandler) GetSELinuxConfigDir() string {
	return "etc/selinux"
}

func (d *ubuntuDistroHandler) SELinuxSupported() bool {
	return false
}

func (d *ubuntuDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	return getSELinuxModeFromLinuxArgs(args)
}

func (d *ubuntuDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.DebianGrubCfgFile)
}

func (d *ubuntuDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.DebianGrubCfgFile)
}

func (d *ubuntuDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs file")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	err := shell.NewExecBuilder("update-initramfs", "-u", "-k", "all").
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs:\n%w", err)
	}

	return nil
}

func (d *ubuntuDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return ErrUbuntuBootLoaderHardReset
}
