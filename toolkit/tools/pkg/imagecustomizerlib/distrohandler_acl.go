// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
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
	return targetos.TargetOsAzureContainerLinux3
}

func (d *aclDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	// ACL Phase 0: only mount/recognize/passthrough is supported.
	// Block operations that would fail with confusing errors later.

	if rc.Storage.CustomizePartitions() {
		return fmt.Errorf("storage repartitioning is not yet supported for ACL")
	}

	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return fmt.Errorf("bootloader hard-reset is not supported on ACL (ACL uses systemd-boot, not GRUB)")
	}

	if rc.Uki != nil && rc.Uki.Mode != imagecustomizerapi.UkiModePassthrough {
		return fmt.Errorf("only UKI passthrough mode is currently supported for ACL (got %q)", rc.Uki.Mode)
	}

	if len(rc.OsKernelCommandLine.ExtraCommandLine) > 0 {
		return fmt.Errorf("kernel command line modification is not yet supported for ACL")
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

func (d *aclDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// See comment in ReadGrub2ConfigFile.
	return "", fs.ErrNotExist
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
	return fmt.Errorf("initramfs regeneration is not yet supported for ACL")
}

func (d *aclDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return fmt.Errorf("bootloader configuration is not yet supported for ACL")
}

func (d *aclDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	// See comment in ReadGrub2ConfigFile.
	return nil, fs.ErrNotExist
}

func (d *aclDistroHandler) ReadKernelCmdlines(bootDir string) (map[string]string, error) {
	kernelToArgs, err := d.ReadGrubConfigLinuxArgs(bootDir)
	if err != nil {
		return nil, err
	}

	return grubKernelArgsToStringMap(kernelToArgs), nil
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
