// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
)

// azureLinuxDistroHandler implements DistroHandler for Azure Linux 2.0 and 3.0.
// Azure Linux 4.0 is handled by azureLinux4DistroHandler.
type azureLinuxDistroHandler struct {
	targetOs       targetos.TargetOs
	packageManager rpmPackageManagerHandler
}

const (
	shimPackageAzl3            = "shim"
	grubEfiPackageAzl3         = "grub2-efi-binary"
	grubEfiNoPrefixPackageAzl3 = "grub2-efi-binary-noprefix"
	grubInstallPackageAzl3     = "grub2"
	grubModulesPackageAzl3     = "grub2-pc"
)

var (
	grubEfiPackagesAzl3     = []string{grubEfiPackageAzl3, grubEfiNoPrefixPackageAzl3}
	systemdBootPackagesAzl3 = []string{systemdBootPackage}
)

func newAzureLinuxDistroHandler(targetOs targetos.TargetOs) *azureLinuxDistroHandler {
	logger.Log.Debugf("Distro handler: Azure Linux (distro='%s', versionid='%s')", targetOs.Distro, targetOs.VersionId)

	return &azureLinuxDistroHandler{
		targetOs:       targetOs,
		packageManager: newTdnfPackageManager(targetOs.VersionId),
	}
}

func (d *azureLinuxDistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *azureLinuxDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	switch d.targetOs.VersionId {
	case "2.0", "3.0":
		// Supported versions

	default:
		err := handleUnsupportedDistroVersion(rc, d.targetOs)
		if err != nil {
			return err
		}
	}

	return nil
}

// ManagePackages handles the complete package management workflow for Azure Linux
func (d *azureLinuxDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

// IsPackageInstalled implements DistroHandler.
func (d *azureLinuxDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *azureLinuxDistroHandler) GetPackageInformation(imageChroot *safechroot.Chroot, packageName string,
) (*PackageVersionInformation, error) {
	return d.packageManager.getPackageInformation(imageChroot, packageName)
}

func (d *azureLinuxDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *azureLinuxDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	bootloaderType, _, err := detectBootloaderType(d, imageChroot, grubEfiPackagesAzl3, systemdBootPackagesAzl3)
	return bootloaderType, err
}

func (d *azureLinuxDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	_, err := validateUkiDependencies(d, imageChroot, systemdBootPackagesAzl3)
	return err
}

func (d *azureLinuxDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *azureLinuxDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	return readBootPartitionUuidFromGrubCfg(filepath.Join(espMountDir, espGrubCfgPathAzl3), bootPartitionRegexAzl3)
}

func (d *azureLinuxDistroHandler) GetSELinuxConfigFile() string {
	return selinuxConfigFileDefault
}

func (d *azureLinuxDistroHandler) UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode,
	imageChroot safechroot.ChrootInterface,
) error {
	return UpdateSELinuxModeInConfigFile(selinuxMode, imageChroot, selinuxConfigFileDefault)
}

func (d *azureLinuxDistroHandler) ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error) {
	return defaultExtractUkiAddonCmdline(addonFilePath, buildDir)
}

func (d *azureLinuxDistroHandler) CleanBootDirectory(imageChroot *safechroot.Chroot) error {
	return defaultCleanBootDirectory(imageChroot, d.GetEspDir(), false)
}

func (d *azureLinuxDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *azureLinuxDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	if d.targetOs.VersionId == "2.0" {
		return getSELinuxModeFromLinuxArgs(args)
	}

	return getSELinuxModeFromLinuxArgsDeferIfMissing(args)
}

func (d *azureLinuxDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinuxDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinuxDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs file")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	var err error
	if d.targetOs.VersionId == "2.0" {
		// The 'mkinitrd' command was removed in Azure Linux 3.0 in favor of using 'dracut' directly.
		err = shell.NewExecBuilder("mkinitrd").
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Chroot(imageChroot.ChrootDir()).
			Execute()
	} else {
		err = shell.NewExecBuilder("dracut", "--force", "--regenerate-all").
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			ErrorStderrLines(1).
			Chroot(imageChroot.ChrootDir()).
			Execute()
	}
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs:\n%w", err)
	}

	return nil
}

func (d *azureLinuxDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	// Azure Linux 3.0+ always uses grub2-mkconfig.
	// The legacy grub config detection logic is only relevant for Azure Linux 2.0.
	// And for new images, always use grub2-mkconfig.
	forceGrubMkconfig := newImage || d.targetOs.VersionId != "2.0"

	return configureDiskBootLoader(imageConnection, rootMountIdType, bootType, selinuxConfig, kernelCommandLine,
		currentSELinuxMode, forceGrubMkconfig, d, resources.AssetsGrubDefFileAzl3, installutils.FedoraGrubEnvRelPath,
		resources.AssetsGrubStubFileAzl3, installutils.GrubStubDirsAzl3, true, /*allowHostGrubInstallFallback*/
		grubInstallPackageAzl3, grubModulesPackageAzl3)
}

func (d *azureLinuxDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	return readKernelCmdlinesFromGrubCfg(bootDir, installutils.FedoraGrubCfgRelPath)
}

func (d *azureLinuxDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	grubCfgPath := filepath.Join(bootDir, installutils.FedoraGrubCfgRelPath)
	return readNonRecoveryKernelCmdlinesFromGrubCfg(grubCfgPath, argNames)
}

func (d *azureLinuxDistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	bootDir := filepath.Join(bootPartitionTmpDir, bootRelativePath)
	grubCfgFullPath := filepath.Join(bootDir, installutils.FedoraGrubCfgRelPath)
	return updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, partitions, buildDir, bootUuid)
}

func (d *azureLinuxDistroHandler) ShimPackage() string {
	return shimPackageAzl3
}

func (d *azureLinuxDistroHandler) GrubEfiPackage() string {
	return grubEfiPackageAzl3
}

func (d *azureLinuxDistroHandler) RootMissingMountDirectories() bool {
	return false
}

func (d *azureLinuxDistroHandler) GetBootArchConfig() (BootFilesArchConfig, error) {
	return bootArchConfigFromMap(bootloaderFilesConfigAzureLinux)
}
