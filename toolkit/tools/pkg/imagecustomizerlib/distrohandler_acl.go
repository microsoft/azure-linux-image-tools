// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
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
	logger.Log.Debugf("Distro handler: ACL (distro='%s', versionid='%s')", targetOs.Distro, targetOs.VersionId)

	return &aclDistroHandler{
		targetOs:       targetOs,
		packageManager: newTdnfPackageManager("3.0"),
	}
}

func (d *aclDistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *aclDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureDistroVersion) {
		return ErrPreviewDistroVersionFeatureRequired
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

	err := d.checkForUnsupportedApis(rc)
	if err != nil {
		return fmt.Errorf("%w (distro='%s', versionid='%s'):\n%w", ErrUnsupportedDistroApi, d.targetOs.Distro,
			d.targetOs.VersionId, err)
	}

	return nil
}

func (d *aclDistroHandler) checkForUnsupportedApis(rc *ResolvedConfig) error {
	if rc.Storage.CustomizePartitions() {
		return fmt.Errorf("storage repartitioning is not yet supported for ACL")
	}

	// The narrow, ACL-only 'acl' partition-grow API is allowed. Growing /usr forces a verity
	// re-seal + UKI rebuild, which only happens under 'storage.reinitializeVerity: all'. Require it
	// explicitly so the output remains a valid, verity-sealed ACL image.
	if rc.Acl != nil && rc.Acl.Usr != nil &&
		rc.Storage.ReinitializeVerity != imagecustomizerapi.ReinitializeVerityTypeAll {
		return fmt.Errorf("'acl.usr' requires 'storage.reinitializeVerity: all' so /usr verity can be "+
			"re-sealed at the new size (and the '%s' preview feature)",
			imagecustomizerapi.PreviewFeatureReinitializeVerity)
	}

	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return fmt.Errorf("bootloader hard-reset is not supported on ACL (ACL uses systemd-boot, not GRUB)")
	}

	for _, configWithBase := range rc.ConfigChain {
		os := configWithBase.Config.OS
		if os == nil {
			continue
		}

		if os.Overlays != nil {
			return fmt.Errorf("overlays are not yet supported for ACL")
		}
	}

	// No up-front --tools-dir gate: ACL-T images ship an in-image tdnf + populated rpmdb, so package ops,
	// UKI 'create' (which validates systemd-boot), and verity (which validates device-mapper) can run
	// against the image chroot directly. Stock ACL images lack these tools; without --tools-dir they get
	// a downstream error from tdnf/rpm instead of a validation-time block.

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

func (d *aclDistroHandler) RemovePackageManagerTools(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot,
) error {
	return rpmRemovePackageManagerTools(imageChroot, d.packageManager, toolsChroot, packageManagementPackagesAzl3)
}

func (d *aclDistroHandler) RemovePackageManagerFiles(ctx context.Context, imageChroot *safechroot.Chroot,
) error {
	return removePackageManagementFiles(imageChroot, packageManagementDirsAzl3)
}

// IsPackageInstalled queries the image's rpm database via tdnf. When toolsChroot is provided, tdnf runs
// inside toolsChroot against the image bind-mounted at /_imageroot (required for stock ACL, which has no
// in-image tdnf). When toolsChroot is nil, tdnf runs directly inside the image chroot (works on ACL-T).
func (d *aclDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot, packageName string,
) (bool, error) {
	return d.packageManager.isPackageInstalled(imageChroot, toolsChroot, packageName)
}

func (d *aclDistroHandler) GetPackageInformation(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	packageName string,
) (*PackageVersionInformation, error) {
	return d.packageManager.getPackageInformation(imageChroot, toolsChroot, packageName)
}

func (d *aclDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) ([]cosiapi.OsPackage, error) {
	// This function is only used for metadata within a COSI file.
	// So, it doesn't matter too much if the package list can't be provided.

	// Check if the rpm command is available.
	// (If toolsChroot is being used, then assume that the rpm command is available.)
	if toolsChroot == nil {
		_, err := shell.LookPathChroot("rpm", imageChroot.ChrootDir())
		if err != nil {
			// RPM command is not found. Return an empty list so the COSI
			// metadata's osPackages field is populated (as []) rather than null.
			return []cosiapi.OsPackage{}, nil
		}
	}

	// Check if the rpm database is available.
	exists, err := file.PathExists(filepath.Join(imageChroot.RootDir(), "/var/lib/rpm"))
	if err != nil {
		// RPM database is not found.
		return nil, fmt.Errorf("failed to check if rpm db exists:\n%w", err)
	}

	if !exists {
		// RPM database doesn't exist. Return an empty list so the COSI
		// metadata's osPackages field is populated (as []) rather than null.
		return []cosiapi.OsPackage{}, nil
	}

	// Get the list of packages.
	packages, err := getAllPackagesFromChrootRpm(imageChroot, toolsChroot)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func (d *aclDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) (cosiapi.BootloaderType, error) {
	// ACL always uses systemd-boot.
	return cosiapi.BootloaderTypeSystemdBoot, nil
}

func (d *aclDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) error {
	_, err := validateUkiDependencies(d, imageChroot, toolsChroot, []string{systemdBootPackage})
	return err
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

func (d *aclDistroHandler) RegenerateInitramfs(ctx context.Context, buildDir string,
	imageChroot *safechroot.Chroot,
) error {
	logger.Log.Infof("Regenerating initramfs for ACL")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	// dracut-install copies files into its staging dir with `cp --preserve=xattr`. Every file on
	// ACL's btrfs /usr carries a btrfs-only xattr ("btrfs.compression"), and reproducing that xattr
	// on the destination fails with ENOTSUP on any non-btrfs filesystem (ext4, tmpfs, ...), which
	// aborts the whole regeneration. So the dracut staging dir must itself be btrfs: back it with a
	// dedicated loopback btrfs image mounted at the staging path for the duration of the regen.
	dracutTmpDirHost := filepath.Join(imageChroot.RootDir(), aclDracutTmpDirName)
	err := os.RemoveAll(dracutTmpDirHost)
	if err != nil {
		return fmt.Errorf("failed to clean dracut tmpdir (%s):\n%w", dracutTmpDirHost, err)
	}
	err = os.MkdirAll(dracutTmpDirHost, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create dracut tmpdir (%s):\n%w", dracutTmpDirHost, err)
	}
	defer os.RemoveAll(dracutTmpDirHost)

	// Create and format the backing btrfs image in the build dir (which has space).
	btrfsImagePath := filepath.Join(buildDir, aclDracutBtrfsImageName)
	err = os.RemoveAll(btrfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to clean dracut btrfs image (%s):\n%w", btrfsImagePath, err)
	}
	err = diskutils.CreateSparseDisk(btrfsImagePath, aclDracutBtrfsImageSizeMiB, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create dracut btrfs image (%s):\n%w", btrfsImagePath, err)
	}
	defer os.Remove(btrfsImagePath)

	loopback, err := safeloopback.NewLoopback(btrfsImagePath)
	if err != nil {
		return fmt.Errorf("failed to attach loopback for dracut btrfs image:\n%w", err)
	}
	defer loopback.Close()

	err = shell.NewExecBuilder("mkfs.btrfs", "-q", loopback.DevicePath()).
		LogLevel(logrus.DebugLevel, logrus.WarnLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to format dracut btrfs image:\n%w", err)
	}

	btrfsMount, err := safemount.NewMount(loopback.DevicePath(), dracutTmpDirHost, "btrfs", 0, "", false /*makeAndDeleteDir*/)
	if err != nil {
		return fmt.Errorf("failed to mount dracut btrfs staging dir (%s):\n%w", dracutTmpDirHost, err)
	}
	defer btrfsMount.Close()

	err = shell.NewExecBuilder("dracut", aclDracutRegenerateArgs()...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs for ACL:\n%w", err)
	}

	err = btrfsMount.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to unmount dracut btrfs staging dir (%s):\n%w", dracutTmpDirHost, err)
	}

	err = loopback.CleanClose()
	if err != nil {
		return fmt.Errorf("failed to detach dracut btrfs loopback:\n%w", err)
	}

	return nil
}

const (
	// aclDracutTmpDirName is a chroot-root-level directory used as dracut's staging tmpdir. It is
	// backed by a dedicated loopback btrfs (see RegenerateInitramfs) because ACL's /usr files carry
	// the btrfs-only "btrfs.compression" xattr, which dracut-install's `cp --preserve=xattr` can
	// only reproduce onto a btrfs destination (ext4/tmpfs return ENOTSUP).
	aclDracutTmpDirName = "ic-dracut-tmp"

	// aclDracutBtrfsImageName is the loopback btrfs image backing the dracut staging dir.
	aclDracutBtrfsImageName = "ic-dracut-tmp.btrfs.img"

	// aclDracutBtrfsImageSizeMiB sizes the backing image generously; --regenerate-all staging is a
	// few hundred MiB, and the sparse image only consumes what is actually written.
	aclDracutBtrfsImageSizeMiB = 2048

	// aclDracutConfDir is ACL's dracut config directory. ACL's config that force-includes the
	// verity/storage modules (add_dracutmodules+=" dm crypt systemd-veritysetup ", nvme/virtio/btrfs
	// drivers, compress="zstd") is the file 99-acl.conf in this directory. dracut's --confdir points
	// directly at the directory it globs "*.conf" files from (it does NOT descend into a
	// dracut.conf.d subdir), and dracut only reads /etc/dracut.conf.d and
	// /usr/lib/dracut/dracut.conf.d by default while the image's /etc is empty. So point --confdir
	// straight at ACL's dracut.conf.d, matching ACL's stock initramfs build.
	aclDracutConfDir = "/usr/share/distro/etc/dracut.conf.d"
)

// aclDracutRegenerateArgs returns the dracut arguments for regenerating all initramfs images. It
// points dracut at ACL's config dir (so the regenerated initramfs pulls in the systemd-veritysetup
// / dm / crypt modules, storage drivers, and zstd compression, matching ACL's stock initramfs) and
// at an explicit staging directory (backed by a dedicated loopback btrfs; see RegenerateInitramfs).
func aclDracutRegenerateArgs() []string {
	return []string{
		"--force", "--regenerate-all",
		"--confdir", aclDracutConfDir,
		"--tmpdir", "/" + aclDracutTmpDirName,
	}
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

func (d *aclDistroHandler) UpdateLiveOSGrubCfgForLiveOS(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, disableSELinux bool, savedConfigs *SavedConfigs,
	kernelVersions []string,
) (string, error) {
	return updateGrubCfgForLiveOS(grubCfgContent, initramfsType, disableSELinux, savedConfigs, kernelVersions)
}

func (d *aclDistroHandler) UpdateLiveOSGrubCfgForIso(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType,
) (string, error) {
	return updateGrubCfgForIso(grubCfgContent, initramfsType)
}

func (d *aclDistroHandler) UpdateLiveOSGrubCfgForPxe(grubCfgContent string,
	initramfsType imagecustomizerapi.InitramfsImageType, bootstrapBaseUrl string, bootstrapFileUrl string,
) (string, error) {
	return updateGrubCfgForPxe(grubCfgContent, initramfsType, bootstrapBaseUrl, bootstrapFileUrl)
}

func (d *aclDistroHandler) FinalizeLiveOSPxeBootConfig(pxeBootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, bootstrapBaseUrl string, bootstrapFileUrl string,
) error {
	return nil
}

func (d *aclDistroHandler) ShimPackage() string {
	// ACL uses systemd-boot + UKI (no shim/grub from a package).
	return ""
}

func (d *aclDistroHandler) GrubEfiPackage() string {
	// ACL does not use grub.
	return ""
}

func (d *aclDistroHandler) LiveOSRequiredPackages() []string {
	return liveOSRequiredPackagesAzl3
}

func (d *aclDistroHandler) LiveOSGrubEfiPrefixDir() string {
	return ""
}

func (d *aclDistroHandler) LiveOSInitrdDracutModules() []string {
	return liveOSInitrdDracutModulesAzl3
}

func (d *aclDistroHandler) RootMissingMountDirectories() bool {
	return true
}

func (d *aclDistroHandler) GetBootArchConfig() (BootFilesArchConfig, error) {
	return bootArchConfigFromMap(bootloaderFilesConfigAzureLinux)
}
