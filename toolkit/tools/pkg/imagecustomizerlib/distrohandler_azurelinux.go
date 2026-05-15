// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

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

// azureLinuxDistroHandler implements distroHandler for Azure Linux
type azureLinuxDistroHandler struct {
	version        string
	packageManager rpmPackageManagerHandler
}

func newAzureLinuxDistroHandler(version string) *azureLinuxDistroHandler {
	var packageManager rpmPackageManagerHandler
	if version == "4.0" {
		packageManager = newDnfPackageManager(version)
	} else {
		packageManager = newTdnfPackageManager(version)
	}

	return &azureLinuxDistroHandler{
		version:        version,
		packageManager: packageManager,
	}
}

func (d *azureLinuxDistroHandler) GetTargetOs() targetos.TargetOs {
	switch d.version {
	case "2.0":
		return targetos.TargetOsAzureLinux2
	case "3.0":
		return targetos.TargetOsAzureLinux3
	case "4.0":
		return targetos.TargetOsAzureLinux4
	default:
		panic("unsupported Azure Linux version: " + d.version)
	}
}

func (d *azureLinuxDistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	if d.version == "4.0" {
		switch rc.OutputImageFormat {
		case imagecustomizerapi.ImageFormatTypeIso, imagecustomizerapi.ImageFormatTypePxeDir, imagecustomizerapi.ImageFormatTypePxeTar:
			return fmt.Errorf("ISO and PXE output formats are not supported for Azure Linux 4.0")
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

func (d *azureLinuxDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *azureLinuxDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface) (BootloaderType, error) {
	var grubEfiPackages []string
	switch d.version {
	case "4.0":
		switch runtime.GOARCH {
		case "amd64":
			grubEfiPackages = []string{grubEfiPackageFedoraAmd64}
		default:
			grubEfiPackages = []string{grubEfiPackageFedoraArm64}
		}
	default:
		grubEfiPackages = grubEfiPackagesAzureLinux3
	}
	return detectBootloaderType(d, imageChroot, grubEfiPackages, d.getSystemdBootPackagesForVersion())
}

func (d *azureLinuxDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	return validateUkiDependencies(d, imageChroot, d.getSystemdBootPackagesForVersion())
}

func (d *azureLinuxDistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *azureLinuxDistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	espGrubCfgPath := espGrubCfgPathAzl3
	bootPartitionRegex := bootPartitionRegexAzl3
	if d.version == "4.0" {
		espGrubCfgPath = espGrubCfgPathAzl4
		bootPartitionRegex = bootPartitionRegexAzl4
	}

	return readBootPartitionUuidFromGrubCfg(filepath.Join(espMountDir, espGrubCfgPath), bootPartitionRegex)
}

func (d *azureLinuxDistroHandler) SELinuxSupported() bool {
	return true
}

func (d *azureLinuxDistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	if d.version == "2.0" {
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
	if d.version == "2.0" {
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
	forceGrubMkconfig := newImage || d.version != "2.0"

	var assetGrubDefFile string
	var assetGrubStubFile string
	var grubStubDirs []string
	switch d.version {
	case "2.0", "3.0":
		assetGrubDefFile = resources.AssetsGrubDefFileAzl3
		assetGrubStubFile = resources.AssetsGrubStubFileAzl3
		grubStubDirs = installutils.GrubStubDirsAzl3
	case "4.0":
		assetGrubDefFile = resources.AssetsGrubDefFileAzl4
		assetGrubStubFile = resources.AssetsGrubStubFileAzl4
		grubStubDirs = installutils.GrubStubDirsAzl4
	default:
		return fmt.Errorf("unsupported Azure Linux version: %s", d.version)
	}

	return configureDiskBootLoader(imageConnection, rootMountIdType, bootType, selinuxConfig, kernelCommandLine,
		currentSELinuxMode, forceGrubMkconfig, d, assetGrubDefFile, installutils.FedoraGrubEnvRelPath,
		assetGrubStubFile, grubStubDirs)
}

func (d *azureLinuxDistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	if d.version == "4.0" {
		// Azure Linux 4.0 uses BLS (Boot Loader Specification).
		return readKernelCmdlinesFromBLSEntries(bootDir)
	}

	// Azure Linux 2.0/3.0 uses grub.cfg with inline linux commands.
	return readKernelCmdlinesFromGrubCfg(bootDir, FedoraGrubCfgPath)
}

func (d *azureLinuxDistroHandler) ReadKernelCmdlines(bootDir string) (map[string]string, error) {
	kernelToArgs, err := d.ReadGrubConfigLinuxArgs(bootDir)
	if err != nil {
		return nil, err
	}

	return grubKernelArgsToStringMap(kernelToArgs), nil
}

func (d *azureLinuxDistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	if d.version == "4.0" {
		return readNonRecoveryKernelCmdlinesFromBLS(bootDir, argNames)
	}

	grubCfgPath := filepath.Join(bootDir, FedoraGrubCfgPath)
	return readNonRecoveryKernelCmdlinesFromGrubCfg(grubCfgPath, argNames)
}

func (d *azureLinuxDistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	bootDir := filepath.Join(bootPartitionTmpDir, bootRelativePath)

	if d.version == "4.0" {
		return updateBLSEntriesForVerity(verityMetadata, bootDir, partitions, buildDir, bootUuid)
	}

	grubCfgFullPath := filepath.Join(bootDir, FedoraGrubCfgPath)
	return updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, partitions, buildDir, bootUuid)
}

func (d *azureLinuxDistroHandler) getSystemdBootPackagesForVersion() []string {
	if d.version == "4.0" {
		return []string{systemdBootPackage, systemdBootUnsignedPackage}
	}
	return []string{systemdBootPackage}
}
