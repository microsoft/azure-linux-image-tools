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
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
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
	if !needPackageCleanup(config) {
		return managePackagesRpm(
			ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
			snapshotTime, d.packageManager)
	}

	// AZL's tdnf is built with HISTORY_DB_DIR=/usr/lib/sysimage/tdnf, so by
	// default it writes its history db under /usr — which on ACL is a read-only
	// dm-verity btrfs volume. Override persistdir to /var/lib/tdnf via the
	// image's tdnf.conf (persistdir can't be set via --setopt). tdnf with
	// --installroot prefers <installroot>/etc/tdnf/tdnf.conf over the host's,
	// so patching the image's config is sufficient.
	imgTdnfConf := filepath.Join(imageChroot.RootDir(), "etc/tdnf/tdnf.conf")
	if err := os.MkdirAll(filepath.Dir(imgTdnfConf), 0o755); err != nil {
		return fmt.Errorf("failed to create tdnf conf dir:\n%w", err)
	}
	if _, err := os.Stat(imgTdnfConf); os.IsNotExist(err) {
		if err := os.WriteFile(imgTdnfConf, []byte("[main]\n"), 0o644); err != nil {
			return fmt.Errorf("failed to create image tdnf.conf:\n%w", err)
		}
	}
	if err := overrideTdnfPersistDir(imgTdnfConf, "/var/lib/tdnf"); err != nil {
		return fmt.Errorf("failed to override tdnf persistdir in image:\n%w", err)
	}

	if toolsChroot != nil {
		// AZL3's tdnf RPM has a post-install scriptlet that runs
		// `tdnf-history-util init` to create history.db, because tdnf >= 3.4.1
		// fails (Error 1802, "history database does not exist") if the DB is
		// absent. ACL ships without tdnf, so that scriptlet never ran on the
		// image. Replicate it here against the install target. The util's -r
		// flag only sets the rpm root (not the history-db path), and the
		// underlying sqlite3_open does not mkdir parents — so we pass the full
		// in-chroot path via -f and pre-create the directory.
		histDir := filepath.Join(imageChroot.RootDir(), "var/lib/tdnf")
		if err := os.MkdirAll(histDir, 0o755); err != nil {
			return fmt.Errorf("failed to create tdnf history dir (%s):\n%w", histDir, err)
		}
		err := shell.NewExecBuilder("/usr/lib/tdnf/tdnf-history-util",
			"-r", "/"+toolsRootImageDir,
			"-f", "/"+toolsRootImageDir+"/var/lib/tdnf/history.db",
			"init").
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(20).
			Chroot(toolsChroot.ChrootDir()).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to initialize tdnf history db in image:\n%w", err)
		}
	}

	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

func (d *aclDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *aclDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	// This function is only used for metadata within a COSI file.
	// So, it doesn't matter too much if the package list can't be provided.

	// Check if the rpm command is available.
	_, err := shell.LookPathChroot("rpm", imageChroot.ChrootDir())
	if err != nil {
		// RPM command is not found.
		return nil, nil
	}

	// Check if the rpm database is available.
	exists, err := file.PathExists(filepath.Join(imageChroot.RootDir(), "/var/lib/rpm"))
	if err != nil {
		// RPM database is not found.
		return nil, fmt.Errorf("failed to check if rpm db exists:\n%w", err)
	}

	if !exists {
		// RPM database doesn't exist.
		return nil, nil
	}

	// Get the list of packages.
	packages, err := getAllPackagesFromChrootRpm(imageChroot)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func (d *aclDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	// ACL always uses systemd-boot.
	return BootloaderTypeSystemdBoot, nil
}

func (d *aclDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	_, err := validateUkiDependencies(d, imageChroot, []string{systemdBootPackage})
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

func (d *aclDistroHandler) RootMissingMountDirectories() bool {
	return true
}

func (d *aclDistroHandler) NeedsToolsChroot() bool {
	return true
}

// overrideTdnfPersistDir ensures tdnf.conf has a `persistdir=<dir>` line in its
// [main] section, replacing any existing `persistdir=` line.
func overrideTdnfPersistDir(confPath, dir string) error {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read tdnf.conf (%s):\n%w", confPath, err)
	}
	want := "persistdir=" + dir
	lines := strings.Split(string(data), "\n")
	mainIdx := -1
	persistIdx := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "[main]" {
			mainIdx = i
		}
		if strings.HasPrefix(t, "persistdir=") {
			persistIdx = i
		}
	}
	switch {
	case persistIdx >= 0:
		if strings.TrimSpace(lines[persistIdx]) == want {
			return nil
		}
		lines[persistIdx] = want
	case mainIdx >= 0:
		lines = append(lines[:mainIdx+1], append([]string{want}, lines[mainIdx+1:]...)...)
	default:
		lines = append([]string{"[main]", want}, lines...)
	}
	logger.Log.Infof("ACL: setting %s in (%s)", want, confPath)
	return os.WriteFile(confPath, []byte(strings.Join(lines, "\n")), 0o644)
}
