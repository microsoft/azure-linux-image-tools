// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
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

	// GetEspDir returns the ESP directory path relative to the image root.
	// For example: "boot/efi" for most distros, "boot" for ACL.
	GetEspDir() string

	// FindBootPartitionUuidFromEsp reads the distro's grub.cfg stub from the already-mounted ESP at espMountDir and
	// returns the UUID of the partition that contains the grub.cfg.
	FindBootPartitionUuidFromEsp(espMountDir string) (string, error)

	// Reports whether SELinux configuration is supported by the tool for this distro.
	SELinuxSupported() bool

	// GetSELinuxModeFromLinuxArgs interprets parsed kernel command-line args and returns the effective SELinux mode
	// the kernel will boot with. Returns SELinuxModeDefault to indicate the caller should fall back to reading
	// /etc/selinux/config.
	GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg) (imagecustomizerapi.SELinuxMode, error)

	// ReadGrub2ConfigFile reads the distro-appropriate grub.cfg file from the chroot.
	ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error)

	// WriteGrub2ConfigFile writes the grub.cfg content to the distro-appropriate path in the chroot.
	WriteGrub2ConfigFile(grub2Config string, imageChroot safechroot.ChrootInterface) error

	// RegenerateInitramfs regenerates the initramfs/initrd using the distro-appropriate tool.
	RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error

	// ConfigureDiskBootLoader performs the full bootloader configuration for a disk image.
	ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
		rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
		selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
		currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool) error

	// ReadGrubConfigLinuxArgs reads kernel command-line arguments from the distro's boot configuration, returning them
	// in parsed grubConfigLinuxArg format.
	ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error)

	// ReadKernelCmdlines reads kernel command-line arguments from the distro's boot configuration (e.g., grub.cfg linux
	// lines or BLS entries). Returns a mapping from kernel filename to the full command-line argument string.
	ReadKernelCmdlines(bootDir string) (map[string]string, error)

	// ReadNonRecoveryKernelCmdlines reads kernel command-line arguments from the boot configuration, excluding
	// recovery entries, and returns only args whose name is in argNames.
	ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error)

	// UpdateBootConfigForVerity updates the boot configuration (grub.cfg or BLS entries) with verity
	// kernel arguments. Each distro handler implements the appropriate strategy.
	UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata, bootPartitionTmpDir string,
		bootRelativePath string, partitions []diskutils.PartitionInfo, buildDir string, bootUuid string) error

	// DefaultMountIdTypeForTargetOs returns the mount identifier type to use for fileSystem in /etc/fstab when the user
	// did not explicitly request one (i.e. they left the global default in place). Implementations must return
	// MountIdentifierTypeDefault to signal "no distro-specific override; let the global default apply", and may return
	// any other type to override it for this particular partition.
	DefaultMountIdTypeForTargetOs(fileSystem imagecustomizerapi.FileSystem) imagecustomizerapi.MountIdentifierType
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
	case targetos.TargetOsAzureLinux4:
		return newAzureLinuxDistroHandler("4.0")
	case targetos.TargetOsAzureContainerLinux3:
		return newAclDistroHandler()
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
