// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
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
	return targetos.TargetOsAcl
}

func (d *aclDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	// TODO: Block unsupported operations (repartitioning, GRUB, initramfs regen, etc.)
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
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
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

func (d *aclDistroHandler) SELinuxSupported() bool {
	return true
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
	return fmt.Errorf("initramfs regeneration is not yet supported for ACL")
}

func (d *aclDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return fmt.Errorf("bootloader configuration is not yet supported for ACL")
}
