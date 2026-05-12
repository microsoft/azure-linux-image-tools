// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
)

// aclDistroHandler implements DistroHandler for Azure Container Linux (ACL).
// ACL uses systemd-boot + UKI (no GRUB) and has an immutable /usr with dm-verity.
type aclDistroHandler struct {
	packageManager rpmPackageManagerHandler
}

func newAclDistroHandler() *aclDistroHandler {
	return &aclDistroHandler{
		packageManager: newTdnfPackageManager("3.0"),
	}
}

func (d *aclDistroHandler) GetTargetOs() targetos.TargetOs {
	return targetos.TargetOsAzureContainerLinux3
}

func (d *aclDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureAzureContainerLinux3) {
		return ErrAzureContainerLinux3PreviewFeatureRequired
	}

	if rc.Storage.CustomizePartitions() {
		return fmt.Errorf("storage repartitioning is not yet supported for ACL")
	}

	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return fmt.Errorf("bootloader hard-reset is not supported on ACL (ACL uses systemd-boot, not GRUB)")
	}

	if rc.Uki != nil && rc.Uki.Mode == imagecustomizerapi.UkiModeModify {
		return fmt.Errorf("UKI modify mode is not yet supported for ACL (addon naming alignment required); use 'create' or 'passthrough'")
	}

	for _, configWithBase := range rc.ConfigChain {
		os := configWithBase.Config.OS
		if os == nil {
			continue
		}

		pkgs := os.Packages
		if len(pkgs.Remove) > 0 || len(pkgs.RemoveLists) > 0 ||
			len(pkgs.Update) > 0 || len(pkgs.UpdateLists) > 0 ||
			pkgs.UpdateExistingPackages {
			return fmt.Errorf("package remove/update is not yet supported for ACL (only install is supported)")
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
	// ACL has no tdnf inside the image. Run tdnf from the host with --installroot
	// pointing to the mounted image root.
	if len(config.Packages.Install) == 0 {
		return nil
	}

	tdnfPath, err := exec.LookPath("tdnf")
	if err != nil {
		return fmt.Errorf("tdnf is required on the host to install packages into ACL images but was not found:\n%w", err)
	}

	logger.Log.Infof("Using host tdnf (%s) with --installroot for ACL package installation", tdnfPath)

	imageRoot := imageChroot.RootDir()

	// Mount RPM sources into the image root.
	var mounts *rpmSourcesMounts
	mounts, err = mountRpmSources(ctx, buildDir, imageChroot, rpmsSources, useBaseImageRpmRepos)
	if err != nil {
		return err
	}
	defer mounts.close()

	// Refresh metadata using host tdnf.
	refreshArgs := []string{
		"check-update", "--refresh", "--assumeyes",
		"--installroot=" + imageRoot,
		"--releasever=" + d.packageManager.getReleaseVersion(),
		"--setopt=reposdir=" + imageRoot + rpmsMountParentDirInChroot,
	}

	err = shell.NewExecBuilder(tdnfPath, refreshArgs...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		// Exit code 100 means updates are available — not an error.
		var exitErr *exec.ExitError
		if !(errors.As(err, &exitErr) && exitErr.ExitCode() == 100) {
			return fmt.Errorf("failed to refresh package metadata for ACL:\n%w", err)
		}
	}

	// Install packages using host tdnf.
	logger.Log.Infof("Installing packages into ACL image (%d): %v", len(config.Packages.Install), config.Packages.Install)

	_, span := startInstallPackagesSpan(ctx, config.Packages.Install)
	defer span.End()

	installArgs := []string{
		"install", "--assumeyes", "--cacheonly",
		"--installroot=" + imageRoot,
		"--releasever=" + d.packageManager.getReleaseVersion(),
		"--setopt=reposdir=" + imageRoot + rpmsMountParentDirInChroot,
	}
	installArgs = append(installArgs, config.Packages.Install...)

	err = shell.NewExecBuilder(tdnfPath, installArgs...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to install packages into ACL image (%v):\n%w", config.Packages.Install, err)
	}

	// Clean cache.
	cleanArgs := []string{
		"clean", "all",
		"--installroot=" + imageRoot,
		"--releasever=" + d.packageManager.getReleaseVersion(),
	}

	err = shell.NewExecBuilder(tdnfPath, cleanArgs...).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to clean package cache for ACL:\n%w", err)
	}

	return nil
}

func (d *aclDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	// ACL does not ship an RPM database or tdnf in the image, so package
	// queries via rpm/tdnf are not possible. Instead, check for known
	// binaries or files that each package provides.
	knownPackageFiles := map[string][]string{
		"systemd-boot": {
			"boot/loader/loader.conf",
		},
		"dracut":        {"usr/bin/dracut"},
		"device-mapper": {"usr/sbin/dmsetup"},
	}

	paths, ok := knownPackageFiles[packageName]
	if !ok {
		logger.Log.Warningf("No known file mapping for package (%s) on ACL; assuming not installed", packageName)
		return false
	}

	for _, relPath := range paths {
		fullPath := filepath.Join(imageChroot.ChrootDir(), relPath)
		_, err := os.Stat(fullPath)
		if err == nil {
			return true
		}
	}
	return false
}

func (d *aclDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *aclDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	return BootloaderTypeSystemdBoot, nil
}

func (d *aclDistroHandler) GetEspDir() string {
	return "boot"
}

func (d *aclDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// ACL does not use GRUB and the EFI System Partition IS the boot partition.
	// Return an empty UUID to signal that the ESP itself is the boot partition.
	return "", nil
}

func (d *aclDistroHandler) GetSELinuxConfigDir() string {
	// ACL uses overlayfs for /etc. At runtime, /etc is composed from the
	// immutable lowerdir and a writable upperdir on the ROOT ext4 partition.
	// When IC mounts the partitions individually (no overlay), /etc/selinux/
	// does not exist on the bare rootfs — the actual SELinux config lives in
	// the overlay lowerdir.
	return "usr/share/distro/etc/selinux"
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

func (d *aclDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return fmt.Errorf("GRUB is not supported on ACL")
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
