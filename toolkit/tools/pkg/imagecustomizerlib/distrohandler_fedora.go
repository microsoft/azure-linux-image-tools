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
	version        string
	packageManager rpmPackageManagerHandler
}

const (
	grubEfiPackageFedoraAmd64 = "grub2-efi-x64"
	grubEfiPackageFedoraArm64 = "grub2-efi-aa64"
	shimPackageFedoraAmd64    = "shim-x64"
	shimPackageFedoraArm64    = "shim-aa64"

	isoBootloaderDirFedora = "/EFI/BOOT"
	bootx64BinaryFedora    = "BOOTX64.EFI"
	bootAA64BinaryFedora   = "BOOTAA64.EFI"
)

// bootloaderFilesConfigFedora is the boot-files map for Fedora-style ESPs (Azure Linux 4 and Fedora).
var bootloaderFilesConfigFedora = map[string]BootFilesArchConfig{
	"amd64": {
		bootBinary:                  bootx64BinaryFedora,
		grubBinary:                  grubx64Binary,
		grubNoPrefixBinary:          grubx64NoPrefixBinary,
		espBootBinaryPath:           espBootloaderDir + "/" + bootx64BinaryFedora,
		espGrubBinaryPath:           espBootloaderDir + "/" + grubx64Binary,
		osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootx64BinaryFedora,
		osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubx64Binary,
		osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubx64NoPrefixBinary,
		isoBootBinaryPath:           isoBootloaderDirFedora + "/" + bootx64BinaryFedora,
		isoGrubBinaryPath:           isoBootloaderDirFedora + "/" + grubx64Binary,
		ukiEfiStubBinary:            ukiEfiStubx64Binary,
		ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubx64Binary,
		ukiAddonStubBinary:          ukiAddonStubx64Binary,
		ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubx64Binary,
	},
	"arm64": {
		bootBinary:                  bootAA64BinaryFedora,
		grubBinary:                  grubAA64Binary,
		grubNoPrefixBinary:          grubAA64NoPrefixBinary,
		espBootBinaryPath:           espBootloaderDir + "/" + bootAA64BinaryFedora,
		espGrubBinaryPath:           espBootloaderDir + "/" + grubAA64Binary,
		osEspBootBinaryPath:         osEspBootloaderDir + "/" + bootAA64BinaryFedora,
		osEspGrubBinaryPath:         osEspBootloaderDir + "/" + grubAA64Binary,
		osEspGrubNoPrefixBinaryPath: osEspBootloaderDir + "/" + grubAA64NoPrefixBinary,
		isoBootBinaryPath:           isoBootloaderDirFedora + "/" + bootAA64BinaryFedora,
		isoGrubBinaryPath:           isoBootloaderDirFedora + "/" + grubAA64Binary,
		ukiEfiStubBinary:            ukiEfiStubAA64Binary,
		ukiEfiStubBinaryPath:        ukiEfiStubDir + "/" + ukiEfiStubAA64Binary,
		ukiAddonStubBinary:          ukiAddonStubAA64Binary,
		ukiAddonStubBinaryPath:      ukiEfiStubDir + "/" + ukiAddonStubAA64Binary,
	},
}

func newFedoraDistroHandler(version string) *fedoraDistroHandler {
	return &fedoraDistroHandler{
		version:        version,
		packageManager: newDnfPackageManager(version),
	}
}

func (d *fedoraDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "42":
		return targetos.TargetOsFedora42
	default:
		panic("unsupported Fedora version: " + d.version)
	}
}

func (d *fedoraDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	switch d.version {
	case "42":
		if !slices.Contains(rc.PreviewFeatures, imagecustomizerapi.PreviewFeatureFedora42) {
			return ErrFedora42PreviewFeatureRequired
		}

	default:
		panic("unsupported Fedora version: " + d.version)
	}

	if rc.HasPackageSnapshotTime() {
		return fmt.Errorf("Package snapshotting API not supported for Fedora:\n%w", ErrUnsupportedFedoraFeature)
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

func (d *fedoraDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *fedoraDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *fedoraDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	var grubEfiPackage string
	switch runtime.GOARCH {
	case "amd64":
		grubEfiPackage = grubEfiPackageFedoraAmd64
	default:
		grubEfiPackage = grubEfiPackageFedoraArm64
	}
	bootloaderType, _, err := detectBootloaderType(d, imageChroot, []string{grubEfiPackage}, []string{systemdBootPackage})
	return bootloaderType, err
}

func (d *fedoraDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	_, err := validateUkiDependencies(d, imageChroot, []string{systemdBootPackage})
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
		installutils.FedoraGrubEnvRelPath, resources.AssetsGrubStubFileAzl4, installutils.GrubStubDirsFedora)
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

func (d *fedoraDistroHandler) GetBootArchConfig() (BootFilesArchConfig, error) {
	return bootArchConfigFromMap(bootloaderFilesConfigFedora)
}
