// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/cosiapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/spdxmanifest"
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

	purlNamespaceAzureLinux = string(targetos.AzureLinux)
	rpmDatabasePathAzl3     = "/var/lib/rpm/rpmdb.sqlite"
)

var (
	grubEfiPackagesAzl3           = []string{grubEfiPackageAzl3, grubEfiNoPrefixPackageAzl3}
	systemdBootPackagesAzl3       = []string{systemdBootPackage}
	liveOSRequiredPackagesAzl3    = []string{"squashfs-tools", "tar", "device-mapper", "curl"}
	liveOSInitrdDracutModulesAzl3 = []string{"dmsquash-live", "livenet", "selinux"}
	packageManagementPackagesAzl3 = []string{"tdnf", "dnf", "rpm"}
	packageManagementDirsAzl3     = []string{
		"/var/lib/rpm",
		"/var/lib/dnf",
		"/usr/lib/sysimage/tdnf",
		"/var/cache/tdnf",
		"/var/cache/dnf",
	}
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

func (d *azureLinuxDistroHandler) RemovePackageManagerTools(ctx context.Context, imageChroot *safechroot.Chroot,
	toolsChroot *safechroot.Chroot,
) error {
	return rpmRemovePackageManagerTools(imageChroot, d.packageManager, toolsChroot, packageManagementPackagesAzl3)
}

func (d *azureLinuxDistroHandler) RemovePackageManagerFiles(ctx context.Context, imageChroot *safechroot.Chroot,
) error {
	return removePackageManagementFiles(imageChroot, packageManagementDirsAzl3)
}

// IsPackageInstalled implements DistroHandler.
func (d *azureLinuxDistroHandler) IsPackageInstalled(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot, packageName string,
) (bool, error) {
	return d.packageManager.isPackageInstalled(imageChroot, toolsChroot, packageName)
}

func (d *azureLinuxDistroHandler) GetPackageInformation(imageChroot *safechroot.Chroot, toolsChroot *safechroot.Chroot,
	packageName string,
) (*PackageVersionInformation, error) {
	return d.packageManager.getPackageInformation(imageChroot, toolsChroot, packageName)
}

func (d *azureLinuxDistroHandler) GetAllPackagesFromChroot(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) ([]cosiapi.OsPackage, error) {
	return getAllPackagesFromChrootRpm(imageChroot, toolsChroot)
}

func (d *azureLinuxDistroHandler) ListInstalledPackagesForSpdx(imageChroot safechroot.ChrootInterface,
) ([]spdxmanifest.Package, error) {
	return listInstalledPackagesRpm(imageChroot, rpmDatabasePathAzl3, purlNamespaceAzureLinux)
}

func (d *azureLinuxDistroHandler) DetectBootloaderType(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) (cosiapi.BootloaderType, error) {
	bootloaderType, _, err := detectBootloaderType(d, imageChroot, toolsChroot, grubEfiPackagesAzl3, systemdBootPackagesAzl3)
	return bootloaderType, err
}

func (d *azureLinuxDistroHandler) ValidateUkiDependencies(imageChroot safechroot.ChrootInterface,
	toolsChroot *safechroot.Chroot,
) error {
	_, err := validateUkiDependencies(d, imageChroot, toolsChroot, systemdBootPackagesAzl3)
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

func (d *azureLinuxDistroHandler) SetupEtcOverlay(ctx context.Context, imageChroot *safechroot.Chroot,
	reinitializeVerity imagecustomizerapi.ReinitializeVerityType,
) (*AclEtcOverlay, error) {
	return nil, nil
}

func (d *azureLinuxDistroHandler) FinalizeUkis(espDir string, addonStubPath string,
	kernelInfo map[string]UkiKernelInfo, buildDir string,
) error {
	return nil
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
	return updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, partitions, buildDir, bootUuid, d)
}

func (d *azureLinuxDistroHandler) UpdateLiveOSGrubCfgForLiveOS(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, disableSELinux bool, savedConfigs *SavedConfigs,
	kernelVersions []string,
) (string, error) {
	return updateGrubCfgForLiveOS(grubCfgContent, initramfsType, disableSELinux, savedConfigs, kernelVersions, d)
}

func (d *azureLinuxDistroHandler) UpdateLiveOSGrubCfgForIso(grubCfgContent string, bootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType,
) (string, error) {
	return updateGrubCfgForIso(grubCfgContent, initramfsType, d)
}

func (d *azureLinuxDistroHandler) UpdateLiveOSGrubCfgForPxe(grubCfgContent string,
	initramfsType imagecustomizerapi.InitramfsImageType, bootstrapBaseUrl string, bootstrapFileUrl string,
) (string, error) {
	return updateGrubCfgForPxe(grubCfgContent, initramfsType, bootstrapBaseUrl, bootstrapFileUrl, d)
}

func (d *azureLinuxDistroHandler) FinalizeLiveOSPxeBootConfig(pxeBootDir string,
	initramfsType imagecustomizerapi.InitramfsImageType, bootstrapBaseUrl string, bootstrapFileUrl string,
) error {
	return nil
}

func (d *azureLinuxDistroHandler) ShimPackage() string {
	return shimPackageAzl3
}

func (d *azureLinuxDistroHandler) GrubEfiPackage() string {
	return grubEfiPackageAzl3
}

func (d *azureLinuxDistroHandler) LiveOSRequiredPackages() []string {
	return liveOSRequiredPackagesAzl3
}

func (d *azureLinuxDistroHandler) LiveOSGrubEfiPrefixDir() string {
	return ""
}

func (d *azureLinuxDistroHandler) LiveOSInitrdDracutModules() []string {
	return liveOSInitrdDracutModulesAzl3
}

func (d *azureLinuxDistroHandler) RootMissingMountDirectories() bool {
	return false
}

func (d *azureLinuxDistroHandler) GetBootArchConfig() (BootFilesArchConfig, error) {
	return bootArchConfigFromMap(bootloaderFilesConfigAzureLinux)
}

// Within the grub.cfg and /etc/default/grub files, Azure Linux 2.0 and 3.0 have a "$kernelopts" value. Image Customizer
// has traditionally added args before "$kernelopts" if it exists. This function helps maintains this behavior for
// back-compat purposes.
func azl3GrubFindCommandLineInsertAt(argTokens []grub.Token, defaultValue int) (int, error) {
	if len(argTokens) <= 0 {
		return defaultValue, nil
	}

	insertAtTokens := []grub.Token(nil)
	for i := range argTokens {
		argToken := argTokens[i]
		if argToken.Type != grub.WORD {
			return 0, fmt.Errorf("unexpected token (%s) in grub config linux command",
				grub.TokenTypeString(argToken.Type))
		}

		if len(argToken.SubWords) == 1 &&
			argToken.SubWords[0].Type == grub.VAR_EXPANSION &&
			argToken.SubWords[0].Value == grubKernelOpts {
			// Found the $kernelopts arg.
			// Any new args to be inserted, will be inserted immediately before this token.
			insertAtTokens = append(insertAtTokens, argToken)
		}
	}

	if len(insertAtTokens) < 1 {
		// Could not find the grubKernelOpts.
		// So, insert at the end.
		return argTokens[len(argTokens)-1].Loc.End.Index, nil
	}
	if len(insertAtTokens) > 1 {
		return 0, fmt.Errorf("too many $%s tokens found in linux command line", grubKernelOpts)
	}

	insertAtToken := insertAtTokens[0]
	insertAt := insertAtToken.Loc.Start.Index
	return insertAt, nil
}

// For Azure Linux 2.0 and 3.0, add the new args to GRUB_CMDLINE_LINUX_DEFAULT instead of GRUB_CMDLINE_LINUX.
// This is mainly done for back-compat purposes.
func (d *azureLinuxDistroHandler) AddExtraCmdlineToDefaultGrub(defaultGrubFileContent string,
	extraCommandLine string,
) (string, error) {
	defaultFileArgs, err := getDefaultGrubFileLinuxArgs(defaultGrubFileContent, nil, azl3GrubFindCommandLineInsertAt)
	if err != nil {
		return "", err
	}

	// Add to GRUB_CMDLINE_LINUX_DEFAULT.
	return addExtraCommandLineToDefaultGrubFileVar(defaultGrubFileContent, defaultFileArgs.recoveryVar,
		extraCommandLine)
}

func (d *azureLinuxDistroHandler) AppendKernelCmdlineArgsAll(inputGrubCfgContent string, extraCommandLine string,
) (string, error) {
	return appendKernelCmdlineArgsAll(inputGrubCfgContent, extraCommandLine, azl3GrubFindCommandLineInsertAt)
}

// Note: We don't need special handling of "$kernelopts" here since new args are added to GRUB_CMDLINE_LINUX instead of
// GRUB_CMDLINE_LINUX_DEFAULT and GRUB_CMDLINE_LINUX shouldn't have a "$kernelopts" value.
func (d *azureLinuxDistroHandler) UpdateDefaultGrubCmdlineArgs(defaultGrubFileContent string,
	argsToRemove []string, newArgs []string,
) (string, error) {
	return updateDefaultGrubCmdlineArgs(defaultGrubFileContent, argsToRemove, newArgs)
}

func (d *azureLinuxDistroHandler) UpdateKernelCmdlineArgsAll(grub2Config string, argsToRemove []string,
	newArgs []string,
) (string, error) {
	return updateKernelCmdlineArgsAll(grub2Config, argsToRemove, newArgs, azl3GrubFindCommandLineInsertAt)
}
