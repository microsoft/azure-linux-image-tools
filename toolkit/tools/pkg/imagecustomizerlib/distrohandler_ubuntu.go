// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
)

// ubuntuDistroHandler implements distroHandler for Ubuntu
type ubuntuDistroHandler struct {
	targetOs targetos.TargetOs
}

const (
	grubEfiPackageDebianAmd64 = "grub-efi-amd64"
	grubEfiPackageDebianArm64 = "grub-efi-arm64"
)

var (
	ErrUbuntuUnsupportedBootloaderHardReset   = NewImageCustomizerError("Validation:UbuntuUnsupportedBootloaderHardReset", "bootloader hard-reset API is not supported yet for Ubuntu images")
	ErrUbuntuUnsupportedDisableBaseImageRepos = NewImageCustomizerError("Validation:UbuntuUnsupportedDisableBaseImageRepos", "disabling base image package repositories is not supported yet for Ubuntu images")
)

func newUbuntuDistroHandler(targetOs targetos.TargetOs) *ubuntuDistroHandler {
	logger.Log.Debugf("Distro handler: Ubuntu (distro='%s', versionid='%s')", targetOs.Distro, targetOs.VersionId)

	return &ubuntuDistroHandler{
		targetOs: targetOs,
	}
}

func (d *ubuntuDistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *ubuntuDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureUbuntu) {
		return ErrUbuntuPreviewFeatureRequired
	}

	switch d.targetOs.VersionId {
	case "22.04", "24.04":
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

func (d *ubuntuDistroHandler) checkForUnsupportedApis(rc *ResolvedConfig) error {
	// Check if Ubuntu is being used with bootloader hard-reset.
	// Ubuntu bootloader config logic is not yet fully implemented.
	if rc.BootLoader.ResetType == imagecustomizerapi.ResetBootLoaderTypeHard {
		return ErrUbuntuUnsupportedBootloaderHardReset
	}

	if len(rc.Options.RpmsSources) > 0 {
		return ErrUnsupportedRpmSources
	}

	// UseBaseImageRpmRepos defaults to true and is only false when the user explicitly
	// passes --disable-base-image-rpm-repos. Ubuntu does not use RPM repos, so disabling
	// them is not meaningful and likely indicates a configuration mistake.
	if !rc.Options.UseBaseImageRpmRepos {
		return ErrUbuntuUnsupportedDisableBaseImageRepos
	}

	if rc.HasPackageSnapshotTime() {
		return ErrUnsupportedPackageSnapshotTime
	}

	return nil
}

// ManagePackages handles the complete package management workflow for Ubuntu
func (d *ubuntuDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesDeb(ctx, config, imageChroot)
}

// IsPackageInstalled checks if a package is installed using dpkg-query.
func (d *ubuntuDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return isPackageInstalledDeb(imageChroot, packageName)
}

func (d *ubuntuDistroHandler) GetPackageInformation(imageChroot *safechroot.Chroot, packageName string,
) (*PackageVersionInformation, error) {
	return nil, fmt.Errorf("Getting package information is not supported yet for Ubuntu images:\n%w",
		ErrUnsupportedUbuntuFeature)
}

func (d *ubuntuDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootDeb(imageChroot)
}

func (d *ubuntuDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	grubEfiPackages := []string{"grub-efi"}
	switch runtime.GOARCH {
	case "amd64":
		grubEfiPackages = append(grubEfiPackages, grubEfiPackageDebianAmd64)
	default:
		grubEfiPackages = append(grubEfiPackages, grubEfiPackageDebianArm64)
	}
	bootloaderType, _, err := detectBootloaderType(d, imageChroot, grubEfiPackages, []string{systemdBootPackage})
	return bootloaderType, err
}

func (d *ubuntuDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	_, err := validateUkiDependencies(d, imageChroot, []string{systemdBootPackage})
	return err
}

func (d *ubuntuDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *ubuntuDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// Reading Ubuntu's grub.cfg stub is not supported, so for now just use Azure Linux 3.0's values.
	return readBootPartitionUuidFromGrubCfg(filepath.Join(espMountDir, espGrubCfgPathAzl3), bootPartitionRegexAzl3)
}

func (d *ubuntuDistroHandler) GetSELinuxConfigFile() string {
	return selinuxConfigFileDefault
}

func (d *ubuntuDistroHandler) UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode,
	imageChroot safechroot.ChrootInterface,
) error {
	return UpdateSELinuxModeInConfigFile(selinuxMode, imageChroot, selinuxConfigFileDefault)
}

func (d *ubuntuDistroHandler) ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error) {
	return defaultExtractUkiAddonCmdline(addonFilePath, buildDir)
}

func (d *ubuntuDistroHandler) CleanBootDirectory(imageChroot *safechroot.Chroot) error {
	return defaultCleanBootDirectory(imageChroot, d.GetEspDir(), false)
}

func (d *ubuntuDistroHandler) SELinuxSupported() bool {
	return false
}

func (d *ubuntuDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	return getSELinuxModeFromLinuxArgs(args)
}

func (d *ubuntuDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.DebianGrubCfgFile)
}

func (d *ubuntuDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.DebianGrubCfgFile)
}

func (d *ubuntuDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs file")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	err := shell.NewExecBuilder("update-initramfs", "-u", "-k", "all").
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs:\n%w", err)
	}

	return nil
}

func (d *ubuntuDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return ErrUbuntuUnsupportedBootloaderHardReset
}

func (d *ubuntuDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	return readKernelCmdlinesFromGrubCfg(bootDir, installutils.DebianGrubCfgRelPath)
}

func (d *ubuntuDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	grubCfgPath := filepath.Join(bootDir, installutils.DebianGrubCfgRelPath)
	return readNonRecoveryKernelCmdlinesFromGrubCfg(grubCfgPath, argNames)
}

func (d *ubuntuDistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	grubCfgFullPath := filepath.Join(bootPartitionTmpDir, bootRelativePath, installutils.DebianGrubCfgRelPath)
	return updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, partitions, buildDir, bootUuid)
}

func (d *ubuntuDistroHandler) ShimPackage() string {
	return "shim"
}

func (d *ubuntuDistroHandler) GrubEfiPackage() string {
	switch runtime.GOARCH {
	case "amd64":
		return grubEfiPackageDebianAmd64
	default:
		return grubEfiPackageDebianArm64
	}
}

func (d *ubuntuDistroHandler) RootMissingMountDirectories() bool {
	return false
}
