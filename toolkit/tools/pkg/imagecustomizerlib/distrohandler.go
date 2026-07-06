// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/version"
)

const (
	packageManagerTDNF = "tdnf"
	packageManagerDNF  = "dnf"
	packageManagerAPT  = "apt-get"

	systemdBootPackage = "systemd-boot"
)

var (
	ErrUnsupportedDistroVersion       = NewImageCustomizerError("Validation:UnsupportedDistroVersion", "base image has unsupported distro version")
	ErrUnsupportedDistroVersionSuffix = fmt.Sprintf("preview feature '%s' may be specified to use unsupported versions", imagecustomizerapi.PreviewFeatureUnsupportedDistroVersion)

	ErrUnsupportedDistroApi           = NewImageCustomizerError("Validation:UnsupportedDistroApi", "unsupported API for distro")
	ErrUnsupportedPackageSnapshotTime = NewImageCustomizerError("Validation:UnsupportedPackageSnapshotTime", "package snapshot time API is not supported")
	ErrUnsupportedRpmSources          = NewImageCustomizerError("Validation:UnsupportedRpmSources", "RPM sources API is not supported")
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

	// IsPackageInstalled reports whether packageName is installed in the image's package database. When toolsChroot is
	// non-nil, implementations that depend on an in-image package manager should run their query inside toolsChroot
	// against installroot=/_imageroot instead of imageChroot — this allows the check to work on images that ship no
	// in-image package manager (e.g. ACL). Implementations that cannot complete the query (e.g. ACL with a nil
	// toolsChroot) must return an error rather than a misleading false.
	IsPackageInstalled(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot, packageName string) (bool, error)

	// GetPackageInformation queries the installed-package database for packageName and returns its parsed information.
	GetPackageInformation(imageChroot *safechroot.Chroot, packageName string) (*PackageVersionInformation, error)

	// Get all installed packages from the chroot.
	// toolsChroot has the same semantics as in IsPackageInstalled.
	GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot) ([]cosiapi.OsPackage, error)

	// Detect the bootloader type installed in the image. toolsChroot has the same semantics as in IsPackageInstalled.
	DetectBootloaderType(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot) (cosiapi.BootloaderType, error)

	// ValidateUkiDependencies verifies that the necessary dependencies for UKI customization are present in the image.
	// toolsChroot has the same semantics as in IsPackageInstalled.
	ValidateUkiDependencies(imageChroot safechroot.ChrootInterface, toolsChroot *safechroot.Chroot) error

	// GetEspDir returns the ESP directory path relative to the image root.
	// For example: "boot/efi" for most distros, "boot" for ACL.
	GetEspDir() string

	// FindBootPartitionUuidFromEsp reads the distro's grub.cfg stub from the already-mounted ESP at espMountDir and
	// returns the UUID of the partition that contains the grub.cfg.
	FindBootPartitionUuidFromEsp(espMountDir string) (string, error)

	// GetSELinuxConfigFile returns the path to the SELinux configuration
	// file relative to the image root.
	GetSELinuxConfigFile() string

	// UpdateSELinuxConfigFile writes the given SELinux mode to the distro's
	// SELinux config file. Implementations may no-op when the config file
	// resides on a read-only partition (e.g. dm-verity /usr on ACL), since
	// the mode is already applied via the kernel command line in that case.
	UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode, imageChroot safechroot.ChrootInterface) error

	// ExtractUkiAddonCmdline returns the current kernel command line from the
	// IC-managed UKI addon at addonFilePath. If the addon does not yet exist,
	// distros that support a first-run addon-creation flow (e.g. ACL) return an
	// empty string; all other distros return an error.
	ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error)

	// CleanBootDirectory removes stale kernel/initramfs/UKI artifacts from /boot
	// after kernel extraction. Distros where /boot IS the ESP (e.g. ACL) only
	// remove kernel and initramfs files; all other entries are preserved.
	CleanBootDirectory(imageChroot *safechroot.Chroot) error

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

	// ReadNonRecoveryKernelCmdlines reads kernel command-line arguments from the boot configuration, excluding
	// recovery entries, and returns only args whose name is in argNames.
	ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error)

	// UpdateBootConfigForVerity updates the boot configuration (grub.cfg or BLS entries) with verity
	// kernel arguments. Each distro handler implements the appropriate strategy.
	UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata, bootPartitionTmpDir string,
		bootRelativePath string, partitions []diskutils.PartitionInfo, buildDir string, bootUuid string) error

	// UpdateLiveOSGrubCfgForLiveOS applies the common LiveOS-compatibility edits to the grub.cfg generation. It is the
	// base that the iso and pxe steps build on.
	UpdateLiveOSGrubCfgForLiveOS(grubCfgContent string, bootDir string,
		initramfsType imagecustomizerapi.InitramfsImageType, disableSELinux bool, savedConfigs *SavedConfigs,
		kernelVersions []string) (string, error)

	// UpdateLiveOSGrubCfgForIso applies the iso-specific edits on top of the LiveOS edits.
	UpdateLiveOSGrubCfgForIso(grubCfgContent string, bootDir string,
		initramfsType imagecustomizerapi.InitramfsImageType) (string, error)

	// ShimPackage returns the package that provides the shim EFI binary for this distro on the current architecture.
	ShimPackage() string

	// GrubEfiPackage returns the package that provides the grub EFI binary for this distro on the current architecture.
	GrubEfiPackage() string

	// LiveOSRequiredPackages returns the packages that must already be installed in the target image for Image
	// Customizer to build a LiveOS bootstrap initrd (the squashfs and dracut live tooling).
	LiveOSRequiredPackages() []string

	// LiveOSGrubEfiPrefixDir returns the ISO-relative directory that this distro's grub EFI binary uses as its baked-in
	// 'prefix', or "" if it doesn't use a baked-in prefix.
	LiveOSGrubEfiPrefixDir() string

	// LiveOSInitrdDracutModules returns the dracut modules to add when building the LiveOS bootstrap initrd.
	LiveOSInitrdDracutModules() []string

	// Distro has a root partition that is missing placeholder directories for special mounts like /dev.
	RootMissingMountDirectories() bool

	// GetBootArchConfig returns the boot-files configuration appropriate for this distro on the current runtime
	// architecture.
	GetBootArchConfig() (BootFilesArchConfig, error)
}

// NewDistroHandler creates a distro handler directly from TargetOs
func NewDistroHandler(targetOs targetos.TargetOs) (DistroHandler, error) {
	switch targetOs.Distro {
	case targetos.Fedora:
		return newFedoraDistroHandler(targetOs), nil

	case targetos.AzureLinux:
		// Future: Once AZL4 is out of preview, switch the unknown/invalid version handling to the AZL4 handler.
		switch {
		case targetOs.Version != nil && targetOs.Version.Ge(version.Version{4, 0}):
			return newAzureLinux4DistroHandler(targetOs), nil

		default:
			return newAzureLinuxDistroHandler(targetOs), nil
		}

	case targetos.AzureContainerLinux:
		return newAclDistroHandler(targetOs), nil

	case targetos.Ubuntu:
		return newUbuntuDistroHandler(targetOs), nil

	default:
		return nil, fmt.Errorf("unsupported target OS: %s %s", targetOs.Distro, targetOs.Version)
	}
}

// NewDistroHandlerFromChroot creates a distro handler by detecting the OS from the chroot
func NewDistroHandlerFromChroot(imageChroot safechroot.ChrootInterface) (DistroHandler, error) {
	targetOs, err := targetos.GetInstalledTargetOs(imageChroot.RootDir())
	if err != nil {
		return nil, fmt.Errorf("failed to determine the target OS:\n%w", err)
	}

	distroHandler, err := NewDistroHandler(targetOs)
	if err != nil {
		return nil, err
	}

	return distroHandler, nil
}

func handleUnsupportedDistroVersion(rc *ResolvedConfig, targetOs targetos.TargetOs) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureUnsupportedDistroVersion) {
		return fmt.Errorf("%w (distro='%s', version='%s'):\n%s", ErrUnsupportedDistroVersion,
			targetOs.Distro, targetOs.VersionId, ErrUnsupportedDistroVersionSuffix)
	}

	return nil
}

// NewDistroHandlerFromInitrd creates a distro handler by detecting the OS from the dracut-emitted initrd-release file
// inside the initramfs at initrdPath. Used in ISO-to-ISO pipelines where no rootfs is mounted yet but a canonical
// distro identity is needed to pick the correct boot-file layout.
func NewDistroHandlerFromInitrd(initrdPath string) (DistroHandler, error) {
	targetOs, err := targetos.GetInitrdTargetOs(initrdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine the target OS from initrd (%s):\n%w", initrdPath, err)
	}

	distroHandler, err := NewDistroHandler(targetOs)
	if err != nil {
		return nil, err
	}

	return distroHandler, nil
}
