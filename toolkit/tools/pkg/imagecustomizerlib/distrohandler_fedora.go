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
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"github.com/sirupsen/logrus"
)

// fedoraDistroHandler implements distroHandler for Fedora
type fedoraDistroHandler struct {
	targetOs       targetos.TargetOs
	packageManager rpmPackageManagerHandler
}

const (
	grubEfiPackageFedoraAmd64 = "grub2-efi-x64"
	grubEfiPackageFedoraArm64 = "grub2-efi-aa64"
	shimPackageFedoraAmd64    = "shim-x64"
	shimPackageFedoraArm64    = "shim-aa64"

	bootx64BinaryFedora  = "BOOTX64.EFI"
	bootAA64BinaryFedora = "BOOTAA64.EFI"

	grubToolsPackageFedora     = "grub2-tools"
	grubPcModulesPackageFedora = "grub2-pc-modules"
)

// bootloaderFilesConfigFedora is the boot-files map for Fedora-style ESPs (Azure Linux 4 and Fedora).
//
// Fedora-style distros do not ship a grub-noprefix binary, so grubNoPrefixBinary and
// osEspGrubNoPrefixBinaryPath are left empty.
var (
	bootloaderFilesConfigFedora = map[string]BootFilesArchConfig{
		"amd64": {
			bootBinary:                  bootx64BinaryFedora,
			grubBinary:                  grubx64Binary,
			grubNoPrefixBinary:          "",
			espBootBinaryPath:           espBootloaderDir + "/" + bootx64BinaryFedora,
			espGrubBinaryPath:           espBootloaderDir + "/" + grubx64Binary,
			osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootx64BinaryFedora,
			osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubx64Binary,
			osEspGrubNoPrefixBinaryPath: "",
			isoBootBinaryPath:           isoBootloaderDir + "/" + bootx64BinaryFedora,
			isoGrubBinaryPath:           isoBootloaderDir + "/" + grubx64Binary,
			ukiEfiStubBinary:            ukiEfiStubx64Binary,
			ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubx64Binary,
			ukiAddonStubBinary:          ukiAddonStubx64Binary,
			ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubx64Binary,
		},
		"arm64": {
			bootBinary:                  bootAA64BinaryFedora,
			grubBinary:                  grubAA64Binary,
			grubNoPrefixBinary:          "",
			espBootBinaryPath:           espBootloaderDir + "/" + bootAA64BinaryFedora,
			espGrubBinaryPath:           espBootloaderDir + "/" + grubAA64Binary,
			osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootAA64BinaryFedora,
			osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubAA64Binary,
			osEspGrubNoPrefixBinaryPath: "",
			isoBootBinaryPath:           isoBootloaderDir + "/" + bootAA64BinaryFedora,
			isoGrubBinaryPath:           isoBootloaderDir + "/" + grubAA64Binary,
			ukiEfiStubBinary:            ukiEfiStubAA64Binary,
			ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubAA64Binary,
			ukiAddonStubBinary:          ukiAddonStubAA64Binary,
			ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubAA64Binary,
		},
	}

	// liveOSRequiredPackagesFedora lists the packages needed to build a LiveOS bootstrap initrd on Fedora-style distros
	// Unlike Azure Linux 3.0, these ship the dmsquash-live/livenet dracut modules in a separate dracut-live package.
	liveOSRequiredPackagesFedora = []string{"squashfs-tools", "tar", "device-mapper", "curl", "dracut-live"}

	// liveOSInitrdDracutModulesFedora lists the dracut modules added to the LiveOS bootstrap initrd on Fedora-style
	// distros. Unlike Azure Linux 3.0, the "selinux" module is omitted, since Fedora-style distros load SELinux policy
	// after switch_root, as indicated by their base dracut's 98selinux check() returning 255, opting it out of normal
	// initramfs builds. Loading it from the initramfs instead would deny execute on the initramfs binaries under
	// enforcing policy.
	liveOSInitrdDracutModulesFedora = []string{"dmsquash-live", "livenet"}
)

func newFedoraDistroHandler(targetOs targetos.TargetOs) *fedoraDistroHandler {
	logger.Log.Debugf("Distro handler: Fedora (distro='%s', versionid='%s')", targetOs.Distro, targetOs.VersionId)

	return &fedoraDistroHandler{
		targetOs:       targetOs,
		packageManager: newDnfPackageManager(targetOs.VersionId),
	}
}

func (d *fedoraDistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *fedoraDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureDistroVersion) {
		return ErrPreviewDistroVersionFeatureRequired
	}

	switch d.targetOs.VersionId {
	case "42":
		// Supported versions.

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

func (d *fedoraDistroHandler) checkForUnsupportedApis(rc *ResolvedConfig) error {
	if rc.HasPackageSnapshotTime() {
		return ErrUnsupportedPackageSnapshotTime
	}

	return nil
}

// ManagePackages handles the complete package management workflow for Fedora
func (d *fedoraDistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager,
	)
}

func (d *fedoraDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot, packageName string,
) (bool, error) {
	return d.packageManager.isPackageInstalled(imageChroot, toolsChroot, packageName)
}

func (d *fedoraDistroHandler) GetPackageInformation(imageChroot *safechroot.Chroot, packageName string,
) (*PackageVersionInformation, error) {
	return d.packageManager.getPackageInformation(imageChroot, packageName)
}

func (d *fedoraDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *fedoraDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) (BootloaderType, error) {
	var grubEfiPackage string
	switch runtime.GOARCH {
	case "amd64":
		grubEfiPackage = grubEfiPackageFedoraAmd64
	default:
		grubEfiPackage = grubEfiPackageFedoraArm64
	}
	bootloaderType, _, err := detectBootloaderType(d, imageChroot, toolsChroot, []string{grubEfiPackage}, []string{systemdBootPackage})
	return bootloaderType, err
}

func (d *fedoraDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) error {
	_, err := validateUkiDependencies(d, imageChroot, toolsChroot, []string{systemdBootPackage})
	return err
}

func (d *fedoraDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *fedoraDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// Reading Fedora's grub.cfg stub is not supported, so for now just use Azure Linux 3.0's values.
	return readBootPartitionUuidFromGrubCfg(filepath.Join(espMountDir, espGrubCfgPathAzl3), bootPartitionRegexAzl3)
}

func (d *fedoraDistroHandler) GetSELinuxConfigFile() string {
	return selinuxConfigFileDefault
}

func (d *fedoraDistroHandler) UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode,
	imageChroot safechroot.ChrootInterface,
) error {
	return UpdateSELinuxModeInConfigFile(selinuxMode, imageChroot, selinuxConfigFileDefault)
}

func (d *fedoraDistroHandler) ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error) {
	return defaultExtractUkiAddonCmdline(addonFilePath, buildDir)
}

func (d *fedoraDistroHandler) CleanBootDirectory(imageChroot *safechroot.Chroot) error {
	return defaultCleanBootDirectory(imageChroot, d.GetEspDir(), false)
}

func (d *fedoraDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *fedoraDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	return getSELinuxModeFromLinuxArgsDeferIfMissing(args)
}

func (d *fedoraDistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *fedoraDistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *fedoraDistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
	logger.Log.Infof("Regenerating initramfs file")

	ctx, span := startRegenerateInitramfsSpan(ctx)
	defer span.End()

	err := shell.NewExecBuilder("dracut", "--force", "--regenerate-all").
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		Chroot(imageChroot.ChrootDir()).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to rebuild initramfs:\n%w", err)
	}

	return nil
}

func (d *fedoraDistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return configureDiskBootLoader(imageConnection, rootMountIdType, bootType, selinuxConfig, kernelCommandLine,
		currentSELinuxMode, true /* forceGrubMkconfig */, d, resources.AssetsGrubDefFileAzl4,
		installutils.FedoraGrubEnvRelPath, resources.AssetsGrubStubFileAzl4, installutils.GrubStubDirsFedora,
		false /* allowHostGrubInstallFallback */, grubToolsPackageFedora, grubPcModulesPackageFedora)
}

func (d *fedoraDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	return readKernelCmdlinesFromBLSEntries(bootDir)
}

func (d *fedoraDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	return readNonRecoveryKernelCmdlinesFromBLS(bootDir, argNames)
}

func (d *fedoraDistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	bootDir := filepath.Join(bootPartitionTmpDir, bootRelativePath)
	return updateBLSEntriesForVerity(verityMetadata, bootDir, partitions, buildDir, bootUuid)
}

func (d *fedoraDistroHandler) UpdateLiveOSGrubCfgForLiveOS(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, disableSELinux bool, savedConfigs *SavedConfigs,
	kernelVersions []string,
) (string, error) {
	return updateLiveOSGrubCfgBLSForLiveOS(grubCfgContent, bootDir, initramfsType, disableSELinux, savedConfigs)
}

func (d *fedoraDistroHandler) UpdateLiveOSGrubCfgForIso(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType,
) (string, error) {
	return updateLiveOSGrubCfgBLSForIso(grubCfgContent, bootDir, initramfsType)
}

func (d *fedoraDistroHandler) ShimPackage() string {
	switch runtime.GOARCH {
	case "amd64":
		return shimPackageFedoraAmd64
	default:
		return shimPackageFedoraArm64
	}
}

func (d *fedoraDistroHandler) GrubEfiPackage() string {
	switch runtime.GOARCH {
	case "amd64":
		return grubEfiPackageFedoraAmd64
	default:
		return grubEfiPackageFedoraArm64
	}
}

func (d *fedoraDistroHandler) LiveOSRequiredPackages() []string {
	return liveOSRequiredPackagesFedora
}

func (d *fedoraDistroHandler) LiveOSGrubEfiPrefixDir() string {
	return "EFI/fedora"
}

func (d *fedoraDistroHandler) LiveOSInitrdDracutModules() []string {
	return liveOSInitrdDracutModulesFedora
}

func (d *fedoraDistroHandler) RootMissingMountDirectories() bool {
	return false
}

func (d *fedoraDistroHandler) GetBootArchConfig() (BootFilesArchConfig, error) {
	return bootArchConfigFromMap(bootloaderFilesConfigFedora)
}
