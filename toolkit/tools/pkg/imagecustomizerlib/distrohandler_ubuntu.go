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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

// ubuntuDistroHandler implements distroHandler for Ubuntu
type ubuntuDistroHandler struct {
	version    string
	grubConfig GrubConfig
}

func newUbuntuDistroHandler(version string) *ubuntuDistroHandler {
	return &ubuntuDistroHandler{
		version: version,
		grubConfig: GrubConfig{
			GrubCfgRelPath:     installutils.UbuntuGrubCfgFile,
			GrubEnvRelPath:     "boot/grub/grubenv",
			GrubMkconfigBinary: "grub-mkconfig",
			SELinuxSupported:   false,
		},
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

	return nil
}

// ManagePackages handles the complete package management workflow for Ubuntu
func (d *ubuntuDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	if len(rpmsSources) > 0 {
		return fmt.Errorf("RPM sources are not supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	// UseBaseImageRpmRepos defaults to true and is only false when the user explicitly
	// passes --disable-base-image-rpm-repos. Ubuntu does not use RPM repos, so disabling
	// them is not meaningful and likely indicates a configuration mistake.
	if !useBaseImageRpmRepos {
		return fmt.Errorf(
			"Disabling base image RPM repositories is not supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	if config.Packages.SnapshotTime != "" {
		return fmt.Errorf("package snapshotTime is not yet supported for Ubuntu images:\n%w",
			ErrUnsupportedUbuntuFeature)
	}

	return managePackagesDeb(ctx, config, imageChroot)
}

// IsPackageInstalled checks if a package is installed using dpkg-query.
func (d *ubuntuDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface,
	packageName string,
) bool {
	return isPackageInstalledDeb(imageChroot, packageName)
}

func (d *ubuntuDistroHandler) GetAllPackagesFromChroot(
	imageChroot safechroot.ChrootInterface,
) ([]OsPackage, error) {
	return getAllPackagesFromChrootDeb(imageChroot)
}

func (d *ubuntuDistroHandler) DetectBootloaderType(
	imageChroot safechroot.ChrootInterface,
) (BootloaderType, error) {
	if d.IsPackageInstalled(imageChroot, "grub-efi-amd64") ||
		d.IsPackageInstalled(imageChroot, "grub-efi-arm64") ||
		d.IsPackageInstalled(imageChroot, "grub-efi") {
		return BootloaderTypeGrub, nil
	}
	if d.IsPackageInstalled(imageChroot, "systemd-boot") {
		return BootloaderTypeSystemdBoot, nil
	}
	return "", fmt.Errorf(
		"unknown bootloader: neither grub-efi-amd64, grub-efi-arm64, nor systemd-boot found")
}

func (d *ubuntuDistroHandler) GetGrubConfig() GrubConfig {
	return d.grubConfig
}

func (d *ubuntuDistroHandler) RegenerateInitrd(ctx context.Context, imageChroot *safechroot.Chroot) error {
	err := imageChroot.UnsafeRun(func() error {
		return shell.ExecuteLiveWithErr(1, "update-initramfs", "-u", "-k", "all")
	})
	if err != nil {
		return fmt.Errorf("failed to run update-initramfs:\n%w", err)
	}

	return nil
}

// ConfigureDiskBootLoader configures the bootloader for Ubuntu images.
// It uses BootCustomizer to update /etc/default/grub and regenerate grub.cfg
// via the distro-appropriate grub-mkconfig binary, and ensures grubenv exists
// at the correct Ubuntu path (/boot/grub/grubenv).
func (d *ubuntuDistroHandler) ConfigureDiskBootLoader(
	imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType,
	bootType imagecustomizerapi.BootType, selinuxConfig imagecustomizerapi.SELinux,
	kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode,
	newImage bool,
) error {
	if bootType == imagecustomizerapi.BootTypeEfi {
		// EFI is supported — continue.
	} else if bootType == imagecustomizerapi.BootTypeLegacy {
		return fmt.Errorf("legacy boot is not supported for Ubuntu images")
	}

	imageChroot := imageConnection.Chroot()

	// Determine the root device identifier string.
	imagerRootMountIdType, err := mountIdentifierTypeToImager(rootMountIdType)
	if err != nil {
		return err
	}

	mountPointMap := make(map[string]string)
	for _, mountPoint := range imageChroot.GetMountPoints() {
		mountPointMap[mountPoint.GetTarget()] = mountPoint.GetSource()
	}

	rootDevPath, ok := mountPointMap["/"]
	if !ok {
		return fmt.Errorf("failed to find root mount point (/)")
	}

	rootDevice, err := installutils.FormatMountIdentifier(
		imagerRootMountIdType, rootDevPath)
	if err != nil {
		return fmt.Errorf("failed to format root device identifier:\n%w", err)
	}

	// Create a BootCustomizer to manage /etc/default/grub and grub.cfg.
	bootCustomizer, err := NewBootCustomizer(
		imageChroot, nil, "", d)
	if err != nil {
		return fmt.Errorf("failed to create boot customizer:\n%w", err)
	}

	// Set the root device in /etc/default/grub (GRUB_DEVICE).
	err = bootCustomizer.SetRootDevice(rootDevice)
	if err != nil {
		return fmt.Errorf("failed to set root device:\n%w", err)
	}

	// Update SELinux kernel command-line args.
	err = bootCustomizer.UpdateSELinuxCommandLine(selinuxConfig.Mode)
	if err != nil {
		return fmt.Errorf("failed to update SELinux command line:\n%w", err)
	}

	// Add extra kernel command-line args.
	err = bootCustomizer.AddKernelCommandLine(kernelCommandLine.ExtraCommandLine)
	if err != nil {
		return fmt.Errorf("failed to add kernel command line:\n%w", err)
	}

	// Write /etc/default/grub and regenerate grub.cfg via grub-mkconfig.
	err = bootCustomizer.WriteToFile(imageChroot)
	if err != nil {
		return fmt.Errorf("failed to write boot configuration:\n%w", err)
	}

	// Ensure grubenv exists at the correct path.
	grubEnvPath := filepath.Join(imageChroot.RootDir(), d.grubConfig.GrubEnvRelPath)
	grubEnvExists, err := file.PathExists(grubEnvPath)
	if err != nil {
		return fmt.Errorf("failed to check grubenv existence:\n%w", err)
	}

	if !grubEnvExists {
		err = file.CopyResourceFile(
			resources.ResourcesFS,
			"assets/grub2/grubenv",
			grubEnvPath,
			0o700,
			0o400)
		if err != nil {
			return fmt.Errorf("failed to install grubenv:\n%w", err)
		}
	}

	return nil
}
