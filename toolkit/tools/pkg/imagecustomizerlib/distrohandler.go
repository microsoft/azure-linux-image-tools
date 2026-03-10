// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

const (
	packageManagerTDNF = "tdnf"
	packageManagerDNF  = "dnf"
	packageManagerAPT  = "apt-get"
)

// PackageType represents the type of package format
type PackageType string

// DistroName represents the distribution name
type DistroName string

const (
	distroNameAzureLinux DistroName = "azurelinux"
	distroNameFedora     DistroName = "fedora"
)

// GrubConfig holds distro-specific grub configuration constants.
type GrubConfig struct {
	// GrubCfgRelPath is the grub config file path relative to root
	// (e.g., "/boot/grub2/grub.cfg" or "/boot/grub/grub.cfg").
	GrubCfgRelPath string

	// GrubEnvRelPath is the grubenv file path relative to root
	// (e.g., "boot/grub2/grubenv" or "boot/grub/grubenv").
	GrubEnvRelPath string

	// GrubMkconfigBinary is the grub-mkconfig binary name
	// ("grub2-mkconfig" for Azure Linux/Fedora, "grub-mkconfig" for Ubuntu).
	GrubMkconfigBinary string

	// SELinuxSupported reports whether SELinux configuration is supported for this distro.
	SELinuxSupported bool
}

// DistroHandler represents the interface for distribution-specific configuration
type DistroHandler interface {
	GetTargetOs() targetos.TargetOs

	// Validates the image config for a distro.
	// This is primarily intended to be used to block unsupported features.
	ValidateConfig(rc *ResolvedConfig) error

	// Package management operations
	ManagePackages(ctx context.Context, buildDir string, baseConfigPath string, config *imagecustomizerapi.OS,
		imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot, rpmsSources []string, useBaseImageRpmRepos bool,
		snapshotTime imagecustomizerapi.PackageSnapshotTime) error

	IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool

	// Get all installed packages from the chroot
	GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error)

	// Detect the bootloader type installed in the image
	DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error)

	// GetGrubConfig returns the distro-specific grub configuration constants.
	GetGrubConfig() GrubConfig

	// RegenerateInitrd regenerates the initramfs using distro-appropriate tooling.
	RegenerateInitrd(ctx context.Context, imageChroot *safechroot.Chroot) error

	// ConfigureDiskBootLoader configures the bootloader for a disk image using
	// distro-appropriate tooling. For Azure Linux and Fedora this delegates to
	// installutils; for Ubuntu it uses BootCustomizer to update /etc/default/grub
	// and regenerate grub.cfg via grub-mkconfig.
	ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
		rootMountIdType imagecustomizerapi.MountIdentifierType,
		bootType imagecustomizerapi.BootType, selinuxConfig imagecustomizerapi.SELinux,
		kernelCommandLine imagecustomizerapi.KernelCommandLine,
		currentSELinuxMode imagecustomizerapi.SELinuxMode,
		newImage bool) error
}

// NewDistroHandlerFromTargetOs creates a distro handler directly from TargetOs
func NewDistroHandlerFromTargetOs(targetOs targetos.TargetOs) DistroHandler {
	switch targetOs {
	case targetos.TargetOsFedora42:
		return newFedoraDistroHandler("42")
	case targetos.TargetOsAzureLinux2:
		return newAzureLinuxDistroHandler("2.0")
	case targetos.TargetOsAzureLinux3:
		return newAzureLinuxDistroHandler("3.0")
	case targetos.TargetOsUbuntu2204:
		return newUbuntuDistroHandler("22.04")
	case targetos.TargetOsUbuntu2404:
		return newUbuntuDistroHandler("24.04")
	default:
		panic("unsupported target OS: " + string(targetOs))
	}
}

// NewDistroHandler creates the appropriate distro handler with version support (legacy)
func NewDistroHandler(distroName string, version string) DistroHandler {
	switch distroName {
	case string(distroNameFedora):
		return newFedoraDistroHandler(version)
	case string(distroNameAzureLinux):
		return newAzureLinuxDistroHandler(version)
	default:
		panic("unsupported distro name: " + distroName)
	}
}

// NewDistroHandlerFromChroot creates a distro handler by detecting the OS from the chroot
func NewDistroHandlerFromChroot(imageChroot safechroot.ChrootInterface) (DistroHandler, error) {
	targetOs, err := targetos.GetInstalledTargetOs(imageChroot.RootDir())
	if err != nil {
		return nil, fmt.Errorf("failed to determine the target OS:\n%w", err)
	}
	return NewDistroHandlerFromTargetOs(targetOs), nil
}
