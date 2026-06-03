// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
)

// aclDistroHandler implements DistroHandler for Azure Container Linux (ACL).
// ACL uses systemd-boot + UKI (no GRUB) and has an immutable /usr with dm-verity.
type aclDistroHandler struct {
	targetOs       targetos.TargetOs
	packageManager rpmPackageManagerHandler
}

func newAclDistroHandler(targetOs targetos.TargetOs) *aclDistroHandler {
	return &aclDistroHandler{
		targetOs:       targetOs,
		packageManager: newTdnfPackageManager("3.0"),
	}
}

func (d *aclDistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *aclDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureAzureContainerLinux) {
		return ErrAzureContainerLinuxPreviewFeatureRequired
	}

	switch d.targetOs.VersionId {
	case "3.0":
		// Supported versions

	default:
		err := handleUnsupportedDistroVersion(rc, d.targetOs)
		if err != nil {
			return err
		}
	}

	if rc.Storage.CustomizePartitions() {
		return fmt.Errorf("storage repartitioning is not yet supported for ACL")
	}

	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return fmt.Errorf("bootloader hard-reset is not supported on ACL (ACL uses systemd-boot, not GRUB)")
	}

	for _, configWithBase := range rc.ConfigChain {
		os := configWithBase.Config.OS
		if os == nil {
			continue
		}

		pkgs := os.Packages
		if len(pkgs.Install) > 0 || len(pkgs.InstallLists) > 0 ||
			len(pkgs.Remove) > 0 || len(pkgs.RemoveLists) > 0 ||
			len(pkgs.Update) > 0 || len(pkgs.UpdateLists) > 0 ||
			pkgs.UpdateExistingPackages {
			return fmt.Errorf("package operations are not yet supported for ACL")
		}

		if os.Overlays != nil {
			return fmt.Errorf("overlays are not yet supported for ACL")
		}
	}

	return nil
}

func (d *aclDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

func (d *aclDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	// ACL images do not ship tdnf or rpm CLIs, so the in-chroot package query
	// used by the base TDNF package manager would always fail. Use the host's
	// rpm binary with --root to query the image's RPM database instead.
	return isPackageInstalledViaHostRpm(imageChroot, packageName)
}

func (d *aclDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	// ACL images do not ship the rpm CLI, so the standard in-chroot
	// rpm -qa query would fail. Try the host's rpm binary with --root
	// to query the image's RPM database. If the DB doesn't exist (common
	// for minimal ACL images that strip the RPM DB), return an empty list.
	packages, err := getAllPackagesFromChrootRpmViaHost(imageChroot)
	if err != nil {
		logger.Log.Warnf("Could not query RPM DB for ACL image, returning empty package list: %v", err)
		return nil, nil
	}
	return packages, nil
}

func (d *aclDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	// ACL always uses systemd-boot + UKI (no GRUB). Hardcode instead of
	// probing via package queries, since ACL images lack tdnf/rpm CLIs.
	return BootloaderTypeSystemdBoot, nil
}

func (d *aclDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	// ACL always ships systemd-boot. No runtime package check needed.
	return nil
}

func (d *aclDistroHandler) GetEspDir() string {
	return "boot"
}

func (d *aclDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// ACL does not use GRUB and the EFI System Partition IS the boot partition.
	// Return an empty UUID to signal that the ESP itself is the boot partition.
	// See comment in ReadGrub2ConfigFile.
	return "", fs.ErrNotExist
}

func (d *aclDistroHandler) GetSELinuxConfigFile() string {
	// ACL uses overlayfs for /etc. At runtime, /etc is composed from the
	// immutable lowerdir and a writable upperdir on the ROOT ext4 partition.
	// When IC mounts the partitions individually (no overlay), /etc/selinux/
	// does not exist on the bare rootfs — the actual SELinux config lives in
	// the overlay lowerdir.
	return "usr/share/distro/etc/selinux/config"
}

func (d *aclDistroHandler) UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode,
	imageChroot safechroot.ChrootInterface,
) error {
	// ACL's /usr is a btrfs+dm-verity volume and is always mounted read-only.
	// The SELinux mode is applied solely via the kernel command line; skip the file update.
	logger.Log.Debugf("Skipping SELinux config file update: /usr is read-only on ACL")
	return nil
}

func (d *aclDistroHandler) ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error) {
	_, statErr := os.Stat(addonFilePath)
	if statErr == nil {
		return extractCmdlineFromSinglePE(addonFilePath, buildDir)
	}
	if os.IsNotExist(statErr) {
		// ACL ships with oem/firstboot addons but no IC-managed addon on first run.
		// Start with empty cmdline; modifyUkiAddon will create the addon.
		logger.Log.Infof("No IC addon found at (%s); a new addon will be created with user-specified args", addonFilePath)
		return "", nil
	}
	return "", fmt.Errorf("failed to stat addon file (%s):\n%w", addonFilePath, statErr)
}

func (d *aclDistroHandler) CleanBootDirectory(imageChroot *safechroot.Chroot) error {
	// ACL mounts the ESP directly at /boot, so /boot IS the ESP.
	// Only remove kernel/initramfs file patterns; preserve all directories and other files.
	return defaultCleanBootDirectory(imageChroot, d.GetEspDir(), true)
}

func (d *aclDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *aclDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	return getSELinuxModeFromLinuxArgs(args)
}

func (d *aclDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	// ACL does not use GRUB. Return empty string with ErrNotExist so callers
	// that tolerate a missing grub.cfg can proceed without error.
	return "", fs.ErrNotExist
}

func (d *aclDistroHandler) WriteGrub2ConfigFile(grub2Config string, imageChroot safechroot.ChrootInterface) error {
	// See comment in ReadGrub2ConfigFile.
	return fs.ErrNotExist
}

func (d *aclDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs for ACL")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	err := shell.NewExecBuilder("dracut", "--force", "--regenerate-all").
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs for ACL:\n%w", err)
	}

	return nil
}

func (d *aclDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return fmt.Errorf("bootloader configuration is not supported on ACL (systemd-boot auto-discovers UKIs)")
}

func (d *aclDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	// See comment in ReadGrub2ConfigFile.
	return nil, fs.ErrNotExist
}

func (d *aclDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	// See comment in ReadGrub2ConfigFile.
	return nil, fs.ErrNotExist
}

func (d *aclDistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	// See comment in ReadGrub2ConfigFile.
	return fs.ErrNotExist
}

func (d *aclDistroHandler) ShimPackage() string {
	// ACL uses systemd-boot + UKI (no shim/grub from a package).
	return ""
}

func (d *aclDistroHandler) GrubEfiPackage() string {
	// ACL does not use grub.
	return ""
}
