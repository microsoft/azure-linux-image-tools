// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// azureLinux4DistroHandler implements DistroHandler for Azure Linux 4.0.
type azureLinux4DistroHandler struct {
	targetOs       targetos.TargetOs
	packageManager rpmPackageManagerHandler
}

const (
	systemdBootUnsignedPackageAzl4 = "systemd-boot-unsigned"
)

var systemdBootPackagesAzl4 = []string{systemdBootPackage, systemdBootUnsignedPackageAzl4}

func newAzureLinux4DistroHandler(targetOs targetos.TargetOs) *azureLinux4DistroHandler {
	logger.Log.Debugf("Distro handler: Azure Linux 4+ (distro='%s', versionid='%s')", targetOs.Distro, targetOs.VersionId)

	return &azureLinux4DistroHandler{
		targetOs:       targetOs,
		packageManager: newDnfPackageManager("4.0"),
	}
}

func (d *azureLinux4DistroHandler) GetTargetOs() targetos.TargetOs {
	return d.targetOs
}

func (d *azureLinux4DistroHandler) ValidateConfig(rc *ResolvedConfig) error {
	switch d.targetOs.VersionId {
	case "4.0":
		// Supported versions

	default:
		err := handleUnsupportedDistroVersion(rc, d.targetOs)
		if err != nil {
			return err
		}
	}

	switch rc.OutputImageFormat {
	case imagecustomizerapi.ImageFormatTypeIso, imagecustomizerapi.ImageFormatTypePxeDir,
		imagecustomizerapi.ImageFormatTypePxeTar:
		return fmt.Errorf("ISO and PXE output formats are not supported for Azure Linux 4.0")
	}

	return nil
}

func (d *azureLinux4DistroHandler) ManagePackages(ctx context.Context, buildDir string, baseConfigPath string,
	config *imagecustomizerapi.OS, imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	rpmsSources []string, useBaseImageRpmRepos bool, snapshotTime imagecustomizerapi.PackageSnapshotTime,
) error {
	return managePackagesRpm(
		ctx, buildDir, baseConfigPath, config, imageChroot, toolsChroot, rpmsSources, useBaseImageRpmRepos,
		snapshotTime, d.packageManager)
}

func (d *azureLinux4DistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface, packageName string) bool {
	return d.packageManager.isPackageInstalled(imageChroot, packageName)
}

func (d *azureLinux4DistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface,
) ([]OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot)
}

func (d *azureLinux4DistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface,
) (BootloaderType, error) {
	var grubEfiPackages []string
	switch runtime.GOARCH {
	case "amd64":
		grubEfiPackages = []string{grubEfiPackageFedoraAmd64}
	default:
		grubEfiPackages = []string{grubEfiPackageFedoraArm64}
	}
	bootloaderType, detectedPackage, err := detectBootloaderType(d, imageChroot, grubEfiPackages,
		systemdBootPackagesAzl4)
	if err != nil {
		return bootloaderType, err
	}
	if bootloaderType == BootloaderTypeSystemdBoot {
		d.warnIfUnsignedSystemdBootPackage(detectedPackage)
	}
	return bootloaderType, nil
}

func (d *azureLinux4DistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface) error {
	detectedSystemdBootPackage, err := validateUkiDependencies(d, imageChroot, systemdBootPackagesAzl4)
	if err != nil {
		return err
	}
	d.warnIfUnsignedSystemdBootPackage(detectedSystemdBootPackage)
	return nil
}

func (d *azureLinux4DistroHandler) GetEspDir() string {
	return "boot/efi"
}

func (d *azureLinux4DistroHandler) FindBootPartitionUuidFromEsp(espMountDir string) (string, error) {
	// Azure Linux 4.0 base images may place the ESP grub.cfg under either
	// EFI/azurelinux (preferred, going forward) or EFI/fedora (legacy). Probe in
	// preference order and use the first one that exists.
	var firstErr error
	for _, relPath := range espGrubCfgPathsAzl4 {
		grubCfgPath := filepath.Join(espMountDir, relPath)
		if _, err := os.Stat(grubCfgPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to stat EFI system partition's grub.cfg file (%s):\n%w", grubCfgPath, err)
			}
			continue
		}
		return readBootPartitionUuidFromGrubCfg(grubCfgPath, bootPartitionRegexAzl4)
	}
	if firstErr != nil {
		return "", firstErr
	}
	return "", fmt.Errorf("failed to find EFI system partition's grub.cfg file under %s (looked for %v)",
		espMountDir, espGrubCfgPathsAzl4)
}

func (d *azureLinux4DistroHandler) GetSELinuxConfigFile() string {
	return selinuxConfigFileDefault
}

func (d *azureLinux4DistroHandler) UpdateSELinuxConfigFile(selinuxMode imagecustomizerapi.SELinuxMode,
	imageChroot safechroot.ChrootInterface,
) error {
	return UpdateSELinuxModeInConfigFile(selinuxMode, imageChroot, selinuxConfigFileDefault)
}

func (d *azureLinux4DistroHandler) ExtractUkiAddonCmdline(addonFilePath string, buildDir string) (string, error) {
	return defaultExtractUkiAddonCmdline(addonFilePath, buildDir)
}

func (d *azureLinux4DistroHandler) CleanBootDirectory(imageChroot *safechroot.Chroot) error {
	return defaultCleanBootDirectory(imageChroot, d.GetEspDir(), false)
}

func (d *azureLinux4DistroHandler) SELinuxSupported() bool {
	return true
}

func (d *azureLinux4DistroHandler) GetSELinuxModeFromLinuxArgs(args []grubConfigLinuxArg,
) (imagecustomizerapi.SELinuxMode, error) {
	return getSELinuxModeFromLinuxArgsDeferIfMissing(args)
}

func (d *azureLinux4DistroHandler) ReadGrub2ConfigFile(imageChroot safechroot.ChrootInterface) (string, error) {
	return readGrub2ConfigFile(imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinux4DistroHandler) WriteGrub2ConfigFile(grub2Config string,
	imageChroot safechroot.ChrootInterface,
) error {
	return writeGrub2ConfigFile(grub2Config, imageChroot, installutils.FedoraGrubCfgFile)
}

func (d *azureLinux4DistroHandler) RegenerateInitramfs(ctx context.Context, imageChroot *safechroot.Chroot) error {
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

func (d *azureLinux4DistroHandler) ConfigureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, newImage bool,
) error {
	return configureDiskBootLoader(imageConnection, rootMountIdType, bootType, selinuxConfig, kernelCommandLine,
		currentSELinuxMode, true /*forceGrubMkconfig*/, d, resources.AssetsGrubDefFileAzl4,
		installutils.FedoraGrubEnvRelPath, resources.AssetsGrubStubFileAzl4, installutils.GrubStubDirsAzl4,
		false /*allowHostGrubInstallFallback*/, grubToolsPackageFedora, grubPcModulesPackageFedora)
}

func (d *azureLinux4DistroHandler) ReadGrubConfigLinuxArgs(bootDir string) (map[string][]grubConfigLinuxArg, error) {
	// Azure Linux 4.0 uses BLS (Boot Loader Specification).
	return readKernelCmdlinesFromBLSEntries(bootDir)
}

func (d *azureLinux4DistroHandler) ReadNonRecoveryKernelCmdlines(bootDir string, argNames []string) (map[string]string, error) {
	return readNonRecoveryKernelCmdlinesFromBLS(bootDir, argNames)
}

func (d *azureLinux4DistroHandler) UpdateBootConfigForVerity(verityMetadata []verityDeviceMetadata,
	bootPartitionTmpDir string, bootRelativePath string, partitions []diskutils.PartitionInfo,
	buildDir string, bootUuid string,
) error {
	bootDir := filepath.Join(bootPartitionTmpDir, bootRelativePath)
	return updateBLSEntriesForVerity(verityMetadata, bootDir, partitions, buildDir, bootUuid)
}

func (d *azureLinux4DistroHandler) warnIfUnsignedSystemdBootPackage(detectedPackage string) {
	if detectedPackage == systemdBootUnsignedPackageAzl4 {
		logger.Log.Warnf("Detected package (%s): Customized image will fail Secure Boot verification", detectedPackage)
	}
}

func (d *azureLinux4DistroHandler) ShimPackage() string {
	switch runtime.GOARCH {
	case "amd64":
		return shimPackageFedoraAmd64
	default:
		return shimPackageFedoraArm64
	}
}

func (d *azureLinux4DistroHandler) GrubEfiPackage() string {
	switch runtime.GOARCH {
	case "amd64":
		return grubEfiPackageFedoraAmd64
	default:
		return grubEfiPackageFedoraArm64
	}
}

func (d *azureLinux4DistroHandler) RootMissingMountDirectories() bool {
	return false
}

func (d *azureLinux4DistroHandler) NeedsToolsChroot() bool {
	return false
}
