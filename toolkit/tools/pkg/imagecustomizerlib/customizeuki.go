// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"gopkg.in/ini.v1"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
)

var (
	// UKI-related errors
	ErrUKIPrepareOS                   = NewImageCustomizerError("UKI:UKIPrepareOS", "failed to prepare OS for uki")
	ErrUKIPackageDependencyValidation = NewImageCustomizerError("UKI:PackageDependencyValidation", "failed to validate package dependencies for uki")
	ErrUKIDirectoryCreate             = NewImageCustomizerError("UKI:DirectoryCreate", "failed to create UKI directories")
	ErrUKIShimFileCopyToTemp          = NewImageCustomizerError("UKI:ShimFileCopyToTemp", "failed to copy shim file to temporary location")
	ErrUKIShimFileCopyFromTemp        = NewImageCustomizerError("UKI:ShimFileCopyFromTemp", "failed to copy shim file from temporary location")
	ErrUKISystemdBootInstall          = NewImageCustomizerError("UKI:SystemdBootInstall", "failed to install systemd-boot")
	ErrUKIRandomSeedRemove            = NewImageCustomizerError("UKI:RandomSeedRemove", "failed to remove random-seed file")
	ErrUKIKernelInitramfsMap          = NewImageCustomizerError("UKI:KernelInitramfsMap", "failed to get kernel to initramfs map")
	ErrUKIFileCopy                    = NewImageCustomizerError("UKI:FileCopy", "failed to copy UKI files")
	ErrUKIKernelCmdlineExtract        = NewImageCustomizerError("UKI:KernelCmdlineExtract", "failed to extract kernel command-line arguments")
	ErrUKICmdlineFileWrite            = NewImageCustomizerError("UKI:CmdlineFileWrite", "failed to write kernel cmdline args JSON")
	ErrUKIExtractComponents           = NewImageCustomizerError("UKI:ExtractComponents", "failed to extract kernel/initramfs from UKI")
	ErrUKICleanOldFiles               = NewImageCustomizerError("UKI:CleanOldFiles", "failed to clean old UKI files")
	ErrUKICleanBootDir                = NewImageCustomizerError("UKI:CleanBootDir", "failed to clean /boot directory")
)

const (
	BootDir            = "boot"
	EspDir             = "boot/efi"
	DefaultGrubCfgPath = "grub2/grub.cfg"
	UkiKernelInfoJson  = "uki-kernel-info.json"
	UkiCmdlineFile     = "uki-cmdline.txt"
	VerityArgsFile     = "verity-args.txt"
	KernelPrefix       = "vmlinuz-"
	UkiBuildDir        = "UkiBuildDir"
	UkiOutputDir       = "EFI/Linux"
)

// Matches UKI filenames like "vmlinuz-<version>.efi"
var ukiNamePattern = regexp.MustCompile(`^vmlinuz-(.+)\.efi$`)

// UkiKernelInfo holds both command line arguments and initramfs name for a UKI kernel
type UkiKernelInfo struct {
	Cmdline   string `json:"cmdline"`
	Initramfs string `json:"initramfs,omitempty"` // Optional: empty in modify mode
}

func baseImageHasUkis(imageChroot *safechroot.Chroot) (bool, error) {
	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	ukiFiles, err := getUkiFiles(espDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for UKI files:\n%w", err)
	}
	return len(ukiFiles) > 0, nil
}

func baseImageHasUkiAddons(espPath string) (bool, error) {
	ukiFiles, err := getUkiFiles(espPath)
	if err != nil {
		return false, fmt.Errorf("failed to get UKI files:\n%w", err)
	}

	if len(ukiFiles) == 0 {
		return false, nil
	}

	// Check if at least one UKI has an addon directory
	for _, ukiFile := range ukiFiles {
		ukiFileName := filepath.Base(ukiFile)
		addonDirPath := filepath.Join(filepath.Dir(ukiFile), fmt.Sprintf("%s.extra.d", ukiFileName))

		if entries, err := os.ReadDir(addonDirPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".addon.efi") {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// validateUkiMode validates the UKI mode against the base image state.
// Rules:
// - If base image has NO UKIs:
//   - No mode specified (os.uki == nil): No UKI created
//   - mode: create: Create UKI
//   - mode: passthrough: FAIL (can't passthrough if no UKIs exist)
//   - mode: modify: FAIL (can't modify if no UKIs exist)
//
// - If base image HAS UKIs:
//   - No mode specified (os.uki == nil): FAIL (must explicitly specify mode)
//   - mode: create: Extract and regenerate UKIs
//   - mode: passthrough: Preserve existing UKIs without modification
//   - mode: modify: Check for addon, modify addon only (preserve main UKI)
func validateUkiMode(imageConnection *imageconnection.ImageConnection, config *imagecustomizerapi.Config) error {
	hasUkis, err := baseImageHasUkis(imageConnection.Chroot())
	if err != nil {
		return err
	}

	if !hasUkis {
		// Base image doesn't have UKIs
		if config.OS != nil && config.OS.Uki != nil {
			// User specified os.uki
			if config.OS.Uki.Mode == imagecustomizerapi.UkiModePassthrough {
				return fmt.Errorf("base image does not contain UKIs but os.uki.mode is set to 'passthrough': " +
					"cannot passthrough UKIs when base image has no UKIs. " +
					"Use mode: create to create UKIs, or omit os.uki entirely",
				)
			}
			if config.OS.Uki.Mode == imagecustomizerapi.UkiModeModify {
				return fmt.Errorf("base image does not contain UKIs but os.uki.mode is set to 'modify': " +
					"cannot modify UKIs when base image has no UKIs. " +
					"Use mode: create to create UKIs from GRUB-based image",
				)
			}
			// mode: create or unspecified (with os.uki present) - both are OK for creating UKIs
		}
		// No os.uki specified - that's fine, no UKI will be created
		return nil
	}

	// Base image has UKIs
	if config.OS == nil || config.OS.Uki == nil {
		return fmt.Errorf("base image contains UKI files but os.uki is not specified: " +
			"when base image has UKIs, you must explicitly specify how to handle them using os.uki.mode " +
			"with one of the following values:\n" +
			"  - 'create': extract and regenerate UKIs with updated configurations\n" +
			"  - 'passthrough': preserve existing UKIs without modification (e.g., to keep signatures intact)\n" +
			"  - 'modify': modify UKI addons only to append kernel command-line arguments")
	}

	if config.OS.Uki.Mode == imagecustomizerapi.UkiModeUnspecified {
		return fmt.Errorf("base image contains UKI files but os.uki.mode is not specified: " +
			"when base image has UKIs, you must explicitly set mode to 'create', 'passthrough', or 'modify'")
	}

	// For modify mode, validate that base image has UKI addons
	if config.OS.Uki.Mode == imagecustomizerapi.UkiModeModify {
		espDir := filepath.Join(imageConnection.Chroot().RootDir(), EspDir)
		hasAddons, err := baseImageHasUkiAddons(espDir)
		if err != nil {
			return fmt.Errorf("failed to check for UKI addons:\n%w", err)
		}
		if !hasAddons {
			return fmt.Errorf("base image has UKI without addons (combined architecture) but os.uki.mode is set to 'modify': " +
				"modify mode requires UKI addon architecture where kernel cmdline is in addon file. " +
				"Use mode: create to regenerate UKIs with addon architecture")
		}
	}

	return nil
}

// extractAndSaveUkiCmdline extracts the kernel cmdline from existing UKI addons and saves them to uki-kernel-info.json.
func extractAndSaveUkiCmdline(buildDir string, imageChroot *safechroot.Chroot) error {
	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	ukiFiles, err := getUkiFiles(espDir)
	if err != nil {
		return fmt.Errorf("failed to get UKI files:\n%w", err)
	}

	if len(ukiFiles) == 0 {
		return fmt.Errorf("no UKI files found for cmdline extraction")
	}

	// Extract cmdline from each UKI addon and create per-kernel entries
	kernelInfo := make(map[string]UkiKernelInfo)
	for _, ukiFile := range ukiFiles {
		ukiFileName := filepath.Base(ukiFile)
		kernelName := strings.TrimSuffix(ukiFileName, ".efi")
		addonDirPath := filepath.Join(filepath.Dir(ukiFile), fmt.Sprintf("%s.extra.d", ukiFileName))
		addonFilePath := filepath.Join(addonDirPath, fmt.Sprintf("%s.addon.efi", kernelName))

		if _, err := os.Stat(addonFilePath); os.IsNotExist(err) {
			return fmt.Errorf("addon file does not exist: %s", addonFilePath)
		}

		cmdline, err := extractCmdlineFromSinglePE(addonFilePath, buildDir)
		if err != nil {
			return fmt.Errorf("failed to extract cmdline from addon (%s):\n%w", addonFilePath, err)
		}

		// In modify mode, we don't have initramfs info, so leave it empty
		kernelInfo[kernelName] = UkiKernelInfo{
			Cmdline:   cmdline,
			Initramfs: "", // Empty for modify mode
		}

		logger.Log.Debugf("Extracted cmdline from UKI (%s): %s", ukiFileName, cmdline)
	}

	// Save to uki-kernel-info.json for consistency with create mode
	ukiKernelInfoPath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)
	err = writeUkiKernelInfoFile(ukiKernelInfoPath, kernelInfo)
	if err != nil {
		return fmt.Errorf("failed to write UKI kernel info file:\n%w", err)
	}

	logger.Log.Infof("Extracted and saved UKI cmdlines to: %s", ukiKernelInfoPath)
	return nil
}

func prepareUki(ctx context.Context, buildDir string, uki *imagecustomizerapi.Uki,
	imageChroot *safechroot.Chroot, distroHandler distroHandler,
) error {
	err := prepareUkiHelper(ctx, buildDir, uki, imageChroot, distroHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIPrepareOS, err)
	}

	return nil
}

func prepareUkiHelper(ctx context.Context, buildDir string, uki *imagecustomizerapi.Uki,
	imageChroot *safechroot.Chroot, distroHandler distroHandler,
) error {
	var err error

	if uki == nil {
		return nil
	}

	// If mode is 'passthrough', skip UKI regeneration to preserve existing UKIs
	if uki.Mode == imagecustomizerapi.UkiModePassthrough {
		logger.Log.Infof("UKI mode is 'passthrough', skipping UKI regeneration")
		return nil
	}

	// If mode is 'modify', skip most UKI preparation but still copy the stub file needed for addon rebuild
	if uki.Mode == imagecustomizerapi.UkiModeModify {
		logger.Log.Infof("UKI mode is 'modify', skipping UKI preparation (will modify addon only)")

		_, bootConfig, err := getBootArchConfig()
		if err != nil {
			return err
		}

		ukiBuildDir := filepath.Join(buildDir, UkiBuildDir)
		err = os.MkdirAll(ukiBuildDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create UKI build directory:\n%w", err)
		}

		err = copyUkiFiles(buildDir, nil, imageChroot, bootConfig, uki)
		if err != nil {
			return err
		}

		// Extract existing cmdline from UKI addon and save to file
		err = extractAndSaveUkiCmdline(buildDir, imageChroot)
		if err != nil {
			return err
		}

		return nil
	}

	logger.Log.Infof("Enabling UKI")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "enable_uki")
	defer span.End()

	// Check UKI dependency packages.
	err = validateUkiDependencies(imageChroot, distroHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIPackageDependencyValidation, err)
	}

	// Create necessary directories for UKI.
	err = createUkiDirectories(buildDir, imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIDirectoryCreate, err)
	}

	// Detect system architecture.
	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}

	// Define the path to the currently installed BOOTX64.EFI in the ESP.
	shimSrcPath := filepath.Join(imageChroot.RootDir(), BootDir, "efi/EFI/BOOT", bootConfig.bootBinary)
	// Define a temporary path to store the backed-up shim binary.
	shimTmpPath := filepath.Join(buildDir, UkiBuildDir, bootConfig.bootBinary)
	// Backup the original shim binary before it gets overwritten by bootctl.
	err = file.Copy(shimSrcPath, shimTmpPath)
	if err != nil {
		return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrUKIShimFileCopyToTemp, shimSrcPath, shimTmpPath, err)
	}

	// This code installs the systemd-boot bootloader into the EFI system partition (ESP).
	// Note: When proper support for systemd-boot is implemented, the `bootctl install` command
	// will likely be invoked as part of the `hardResetBootLoader()` function under BootLoader structure.
	//
	// The command being executed is:
	//     bootctl install --no-variables
	// This performs the following steps:
	//   1. Creates the necessary directories in the ESP, such as:
	//        - /boot/efi/EFI/systemd
	//        - /boot/efi/loader
	//        - /boot/efi/loader/entries
	//   2. Copies the systemd bootloader binary from the host filesystem to the ESP:
	//        - Copies /usr/lib/systemd/boot/efi/systemd-bootx64.efi to /boot/efi/EFI/systemd/systemd-bootx64.efi
	//        - Copies /usr/lib/systemd/boot/efi/systemd-bootx64.efi to /boot/efi/EFI/BOOT/BOOTX64.EFI
	//          (This second location serves as the fallback bootloader entry, adhering to UEFI conventions.)
	//   3. Writes a random seed to /boot/efi/loader/random-seed. This is used by the bootloader to initialize randomness.
	//      This file is removed below to avoid initializing the same seed in all instances.
	//
	// The "--no-variables" flag ensures that the command does not modify UEFI NVRAM boot variables. Instead, it relies
	// on the bootloader binaries being present in the ESP for booting.
	err = imageChroot.UnsafeRun(func() error {
		return shell.ExecuteLiveWithErr(1, "bootctl", "install", "--no-variables")
	})
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKISystemdBootInstall, err)
	}

	// Restore the original signed shim binary to BOOTX64.EFI.
	// This ensures that the Secure Boot chain is preserved,
	// because shim (not systemd-boot) must be the entry point under EFI/BOOT.
	err = file.Copy(shimTmpPath, shimSrcPath)
	if err != nil {
		return fmt.Errorf("%w (source='%s', destination='%s'):\n%w", ErrUKIShimFileCopyFromTemp, shimTmpPath, shimSrcPath, err)
	}

	// The "--random-seed=no" flag is preferred to disable this behavior, but it requires systemd version 257 or later.
	// Since AZL 3.0 uses version 255, we manually remove the random-seed file here for now.
	randomSeedPath := filepath.Join(imageChroot.RootDir(), "/boot/efi/loader/random-seed")
	if err := file.RemoveFileIfExists(randomSeedPath); err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrUKIRandomSeedRemove, randomSeedPath, err)
	}

	// Map kernels and initramfs.
	bootDir := filepath.Join(imageChroot.RootDir(), BootDir)
	kernelToInitramfs, err := getKernelToInitramfsMap(bootDir)
	if err != nil {
		return fmt.Errorf("%w (bootDir='%s'):\n%w", ErrUKIKernelInitramfsMap, bootDir, err)
	}

	// Copy UKI-specific files such as kernel, initramfs, and UKI stub file.
	err = copyUkiFiles(buildDir, kernelToInitramfs, imageChroot, bootConfig, uki)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIFileCopy, err)
	}

	// Extract kernel command line arguments from either grub.cfg or UKI.
	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	kernelToArgs, err := extractKernelToArgs(espDir, bootDir, buildDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIKernelCmdlineExtract, err)
	}

	err = cleanBootDirectory(imageChroot)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKICleanBootDir, err)
	}

	// Combine kernel-to-initramfs mapping and kernel command line arguments into a single structure.
	kernelInfo := make(map[string]UkiKernelInfo)

	for kernel, initramfs := range kernelToInitramfs {
		cmdline, exists := kernelToArgs[kernel]
		if !exists {
			return fmt.Errorf("no command line arguments found for kernel (%s)", kernel)
		}

		kernelInfo[kernel] = UkiKernelInfo{
			Cmdline:   cmdline,
			Initramfs: initramfs,
		}
	}

	// Dump kernel information to a file in buildDir.
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)
	err = writeUkiKernelInfoFile(cmdlineFilePath, kernelInfo)
	if err != nil {
		return fmt.Errorf("%w (path='%s'):\n%w", ErrUKICmdlineFileWrite, cmdlineFilePath, err)
	}

	return nil
}

func validateUkiDependencies(imageChroot *safechroot.Chroot, distroHandler distroHandler) error {
	// The following packages are required for the UKI feature:
	// - "systemd-boot": Checked as a package dependency here to ensure installation,
	//    but additional configuration is handled elsewhere in the UKI workflow.
	requiredRpms := []string{"systemd-boot"}

	// Iterate over each required package and check if it's installed.
	for _, pkg := range requiredRpms {
		logger.Log.Debugf("Checking if package (%s) is installed", pkg)
		installed := distroHandler.isPackageInstalled(imageChroot, pkg)
		if !installed {
			return fmt.Errorf("package (%s) is not installed:\n"+
				"the following packages must be installed to use Uki: (%v)", pkg, requiredRpms)
		}
	}

	return nil
}

func createUkiDirectories(buildDir string, imageChroot *safechroot.Chroot) error {
	dirsToCreate := []string{
		filepath.Join(imageChroot.RootDir(), BootDir, "efi", UkiOutputDir),
		filepath.Join(buildDir, UkiBuildDir),
	}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create directory (%s):\n%w", dir, err)
		}
	}

	return nil
}

func copyUkiFiles(buildDir string, kernelToInitramfs map[string]string, imageChroot *safechroot.Chroot,
	bootConfig BootFilesArchConfig, uki *imagecustomizerapi.Uki,
) error {
	// Both create and modify modes need the stub file
	filesToCopy := map[string]string{
		filepath.Join(imageChroot.RootDir(), bootConfig.ukiEfiStubBinaryPath): filepath.Join(buildDir, UkiBuildDir, bootConfig.ukiEfiStubBinary),
	}

	// Create mode needs additional files (os-release, kernels, initramfs)
	if uki == nil || uki.Mode != imagecustomizerapi.UkiModeModify {
		filesToCopy[filepath.Join(imageChroot.RootDir(), "/etc/os-release")] = filepath.Join(buildDir, UkiBuildDir, "os-release")

		for kernel, initramfs := range kernelToInitramfs {
			kernelSource := filepath.Join(imageChroot.RootDir(), BootDir, kernel)
			kernelDest := filepath.Join(buildDir, UkiBuildDir, kernel)
			filesToCopy[kernelSource] = kernelDest

			initramfsSource := filepath.Join(imageChroot.RootDir(), BootDir, initramfs)
			initramfsDest := filepath.Join(buildDir, UkiBuildDir, initramfs)
			filesToCopy[initramfsSource] = initramfsDest
		}
	}

	// Single copy loop for all files
	for src, dest := range filesToCopy {
		err := file.Copy(src, dest)
		if err != nil {
			return fmt.Errorf("failed to copy UKI files:\n%w", err)
		}
	}

	return nil
}

func getKernelToInitramfsMap(bootDir string) (map[string]string, error) {
	kernelToInitramfs, err := findKernelsAndInitramfs(bootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find kernels and initramfs:\n%w", err)
	}
	return kernelToInitramfs, nil
}

func findKernelsAndInitramfs(bootDir string) (map[string]string, error) {
	kernelToInitramfs := make(map[string]string)

	dirEntries, err := os.ReadDir(bootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read boot directory (%s):\n%w", bootDir, err)
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "vmlinuz-") {
			kernelName := entry.Name()
			kernelVersion, err := getKernelVersion(kernelName)
			if err != nil {
				return nil, err
			}
			initramfsName := fmt.Sprintf("initramfs-%s.img", kernelVersion)
			initramfsPath := filepath.Join(bootDir, initramfsName)

			exists, err := file.PathExists(initramfsPath)
			if err != nil {
				return nil, fmt.Errorf("error checking existence of initramfs (%s):\n%w", initramfsPath, err)
			}
			if !exists {
				return nil, fmt.Errorf("missing initramfs for kernel: (%s), expected at (%s)", kernelName, initramfsPath)
			}

			kernelToInitramfs[kernelName] = initramfsName
		}
	}

	if len(kernelToInitramfs) == 0 {
		return nil, fmt.Errorf("no kernel images found in boot directory (%s)", bootDir)
	}

	return kernelToInitramfs, nil
}

// createUkiInModifyMode modifies UKI addons without touching the main UKI files.
func createUkiInModifyMode(ctx context.Context, rc *ResolvedConfig) error {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_uki_modify_mode")
	defer span.End()

	var err error

	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}

	loopback, err := safeloopback.NewLoopback(rc.RawImageFile)
	if err != nil {
		return fmt.Errorf("failed to connect to image file:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return err
	}

	systemBootPartitionTmpDir := filepath.Join(rc.BuildDirAbs, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path, systemBootPartitionTmpDir, systemBootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	// Get all UKI files from the ESP partition
	// Note: getUkiFiles will append UkiOutputDir internally, so we just pass the mount point
	ukiFiles, err := getUkiFiles(systemBootPartitionTmpDir)
	if err != nil {
		return fmt.Errorf("failed to get UKI files:\n%w", err)
	}

	logger.Log.Infof("Found %d UKI files in ESP partition", len(ukiFiles))
	for i, ukiFile := range ukiFiles {
		logger.Log.Debugf("UKI file [%d]: %s", i, ukiFile)
	}

	stubPath := filepath.Join(rc.BuildDirAbs, UkiBuildDir, bootConfig.ukiEfiStubBinary)

	// Apply verity args to UKI cmdline if they exist
	err = applyVerityArgsToUkiCmdline(rc.BuildDirAbs)
	if err != nil {
		return fmt.Errorf("failed to apply verity args to UKI cmdline:\n%w", err)
	}

	// Process each UKI file - regenerate its addon with updated cmdline
	for _, ukiFile := range ukiFiles {
		err := modifyUkiAddon(ukiFile, stubPath, rc)
		if err != nil {
			return fmt.Errorf("failed to modify UKI addon for (%s):\n%w", filepath.Base(ukiFile), err)
		}
	}

	err = systemBootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// modifyUkiAddon regenerates a UKI addon with updated kernel command line arguments.
// The cmdline has already been modified by BootCustomizer and saved to uki-kernel-info.json.
func modifyUkiAddon(ukiFilePath string, stubPath string, rc *ResolvedConfig) error {
	ukiFileName := filepath.Base(ukiFilePath)

	// Extract kernel name from UKI filename (e.g., "vmlinuz-6.6.x.x.efi" -> "vmlinuz-6.6.x.x")
	kernelName := strings.TrimSuffix(ukiFileName, ".efi")

	// Read the final cmdline from uki-kernel-info.json (already modified by BootCustomizer)
	ukiKernelInfoPath := filepath.Join(rc.BuildDirAbs, UkiBuildDir, UkiKernelInfoJson)
	kernelInfo, err := readUkiKernelInfoFile(ukiKernelInfoPath)
	if err != nil {
		return fmt.Errorf("failed to read UKI kernel info:\n%w", err)
	}

	info, exists := kernelInfo[kernelName]
	if !exists {
		return fmt.Errorf("no cmdline found for kernel (%s) in uki-kernel-info.json", kernelName)
	}
	modifiedCmdline := strings.TrimSpace(info.Cmdline)

	// Rebuild the addon with modified cmdline
	addonDirPath := filepath.Join(filepath.Dir(ukiFilePath), fmt.Sprintf("%s.extra.d", ukiFileName))
	addonFullPath := filepath.Join(addonDirPath, fmt.Sprintf("%s.addon.efi", kernelName))

	ukifyCmd := []string{
		"build",
		fmt.Sprintf("--cmdline=%s", modifiedCmdline),
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", addonFullPath),
	}

	err = shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to rebuild UKI addon:\n%w", err)
	}

	logger.Log.Infof("Successfully modified UKI addon: (%s)", addonFullPath)
	return nil
}

// applyVerityArgsToUkiCmdline reads verity-args.txt and appends the args to UKI cmdline for each kernel.
// Works for both create and modify modes by using uki-kernel-info.json.
func applyVerityArgsToUkiCmdline(buildDir string) error {
	verityArgsPath := filepath.Join(buildDir, UkiBuildDir, VerityArgsFile)

	// Check if verity-args.txt exists
	if _, err := os.Stat(verityArgsPath); os.IsNotExist(err) {
		// No verity args to apply
		return nil
	}

	// Read verity args
	verityArgsBytes, err := os.ReadFile(verityArgsPath)
	if err != nil {
		return fmt.Errorf("failed to read verity args file:\n%w", err)
	}
	verityArgs := strings.TrimSpace(string(verityArgsBytes))

	if verityArgs == "" {
		return nil
	}

	// Update uki-kernel-info.json (used by both create and modify modes)
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)
	kernelInfo, err := readUkiKernelInfoFile(cmdlineFilePath)
	if err != nil {
		return fmt.Errorf("failed to read kernel info file:\n%w", err)
	}

	// Append verity args to each kernel's cmdline
	for kernel, info := range kernelInfo {
		updatedCmdline := strings.TrimSpace(info.Cmdline)
		if updatedCmdline != "" {
			updatedCmdline += " "
		}
		updatedCmdline += verityArgs

		kernelInfo[kernel] = UkiKernelInfo{
			Cmdline:   updatedCmdline,
			Initramfs: info.Initramfs,
		}
	}

	// Write updated kernel info back
	err = writeUkiKernelInfoFile(cmdlineFilePath, kernelInfo)
	if err != nil {
		return fmt.Errorf("failed to write updated kernel info:\n%w", err)
	}

	logger.Log.Infof("Applied verity args to UKI cmdline for all kernels")
	return nil
}

func createUki(ctx context.Context, rc *ResolvedConfig) error {
	logger.Log.Infof("Creating UKIs")

	// If mode is 'passthrough', skip UKI creation to preserve existing UKIs
	if rc.Uki != nil && rc.Uki.Mode == imagecustomizerapi.UkiModePassthrough {
		logger.Log.Infof("UKI mode is 'passthrough', skipping UKI creation")
		return nil
	}

	// If mode is 'modify', only modify UKI addons (preserve main UKI)
	if rc.Uki != nil && rc.Uki.Mode == imagecustomizerapi.UkiModeModify {
		logger.Log.Infof("UKI mode is 'modify', modifying UKI addons only")
		err := createUkiInModifyMode(ctx, rc)
		if err != nil {
			return fmt.Errorf("failed to modify UKI addons in modify mode:\n%w", err)
		}
		return nil
	}

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_uki")
	defer span.End()

	var err error

	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}

	loopback, err := safeloopback.NewLoopback(rc.RawImageFile)
	if err != nil {
		return fmt.Errorf("failed to connect to image file to provision UKI:\n%w", err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return err
	}

	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return err
	}

	systemBootPartitionTmpDir := filepath.Join(rc.BuildDirAbs, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path, systemBootPartitionTmpDir, systemBootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	ukiOutputFullPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir)
	err = cleanUkiDirectory(ukiOutputFullPath)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKICleanOldFiles, err)
	}

	stubPath := filepath.Join(rc.BuildDirAbs, UkiBuildDir, bootConfig.ukiEfiStubBinary)
	osSubreleaseFullPath := filepath.Join(rc.BuildDirAbs, UkiBuildDir, "os-release")
	cmdlineFilePath := filepath.Join(rc.BuildDirAbs, UkiBuildDir, UkiKernelInfoJson)

	// Apply verity args to kernel info if they exist
	err = applyVerityArgsToUkiCmdline(rc.BuildDirAbs)
	if err != nil {
		return fmt.Errorf("failed to apply verity args to kernel info:\n%w", err)
	}

	// Read the kernel information (kernels, initramfs, and command line args) from the file created during prepareUki.
	kernelInfo, err := readUkiKernelInfoFile(cmdlineFilePath)
	if err != nil {
		return err
	}

	for kernel, info := range kernelInfo {
		err := buildUki(kernel, info.Initramfs, info.Cmdline, osSubreleaseFullPath, stubPath, rc.BuildDirAbs,
			systemBootPartitionTmpDir,
		)
		if err != nil {
			return fmt.Errorf("failed to build UKI for kernel (%s):\n%w", kernel, err)
		}
	}

	err = cleanupUkiBuildDir(rc.BuildDirAbs)
	if err != nil {
		return fmt.Errorf("Error during cleanup UKI build dir:\n%w", err)
	}

	err = systemBootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

func extractKernelToArgs(espPath string, bootDir string, buildDir string) (map[string]string, error) {
	// Try extracting from grub.cfg first
	grubCfgPath := filepath.Join(bootDir, DefaultGrubCfgPath)
	kernelToArgs, err := extractKernelToArgsFromGrub(grubCfgPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to extract kernel args from grub.cfg:\n%w", err)
	} else if !errors.Is(err, fs.ErrNotExist) && len(kernelToArgs) > 0 {
		// Successfully extracted kernel cmdline from grub.cfg
		return kernelToArgs, nil
	}

	// Fallback to extracting from UKI
	kernelToArgs, err = extractKernelCmdlineFromUkiEfis(espPath, buildDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to extract kernel args from UKI:\n%w", err)
	} else if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no kernel arguments found from either grub.cfg or UKI")
	}

	if len(kernelToArgs) == 0 {
		return nil, fmt.Errorf("no kernel command-line arguments extracted from UKI files in (%s)", espPath)
	}

	return kernelToArgs, nil
}

// Note: This function will be optimized by leveraging the internal functions
// under grubcfgutils.go when implementing bootloader customization.
func extractKernelToArgsFromGrub(grubCfgPath string) (map[string]string, error) {
	kernelToArgs, err := extractKernelCmdlineFromGrubFile(grubCfgPath)
	if err != nil {
		return nil, err
	}

	kernelToArgsString := make(map[string]string)
	for kernel, args := range kernelToArgs {
		normalizedKernel := kernel
		// Normalize kernel path: strip "boot/" prefix if present. When there's
		// no separate /boot partition, grub.cfg has paths like
		// "boot/vmlinuz-*" but kernel discovery returns just "vmlinuz-*".
		normalizedKernel = strings.TrimPrefix(kernel, "boot/")

		filteredArgs := []string(nil)
		for _, arg := range args {
			if arg.ValueHasVarExpansion {
				// Ignore tokens with $ vars.
				continue
			}

			filteredArgs = append(filteredArgs, arg.Arg)
		}

		filteredArgsString := GrubArgsToString(filteredArgs)
		kernelToArgsString[normalizedKernel] = filteredArgsString
	}

	return kernelToArgsString, nil
}

func buildUki(kernel string, initramfs string, kernelArgs string, osSubreleaseFullPath string,
	stubPath string, buildDir string, systemBootPartitionTmpDir string,
) error {
	kernelVersion, err := getKernelVersion(kernel)
	if err != nil {
		return err
	}

	// Build main UKI
	err = buildMainUki(kernel, initramfs, osSubreleaseFullPath, stubPath, buildDir, systemBootPartitionTmpDir, kernelVersion)
	if err != nil {
		return fmt.Errorf("failed to build main UKI:\n%w", err)
	}

	// Build UKI addon
	err = buildUkiAddon(kernel, kernelArgs, stubPath, systemBootPartitionTmpDir)
	if err != nil {
		return fmt.Errorf("failed to build UKI addon:\n%w", err)
	}

	return nil
}

func buildMainUki(kernel string, initramfs string, osSubreleaseFullPath string, stubPath string,
	buildDir string, systemBootPartitionTmpDir string, kernelVersion string,
) error {
	mainUkiConfigPath := filepath.Join(buildDir, UkiBuildDir, fmt.Sprintf("ukify_main_%s.conf", kernelVersion))

	// Create the INI file for main UKI.
	cfg := ini.Empty()
	section, err := cfg.NewSection("UKI")
	if err != nil {
		return fmt.Errorf("failed to create INI section:\n%w", err)
	}

	// Add Linux, OSRelease, and Initrd to main UKI.
	_, err = section.NewKey("Linux", filepath.Join(buildDir, UkiBuildDir, kernel))
	if err != nil {
		return fmt.Errorf("failed to add 'Linux' key to INI file:\n%w", err)
	}

	_, err = section.NewKey("OSRelease", fmt.Sprintf("@%s", osSubreleaseFullPath))
	if err != nil {
		return fmt.Errorf("failed to add 'OSRelease' key to INI file:\n%w", err)
	}

	_, err = section.NewKey("Initrd", filepath.Join(buildDir, UkiBuildDir, initramfs))
	if err != nil {
		return fmt.Errorf("failed to add 'Initrd' key to INI file:\n%w", err)
	}

	// Save the INI file.
	err = cfg.SaveTo(mainUkiConfigPath)
	if err != nil {
		return fmt.Errorf("failed to save main UKI INI file for kernel (%s):\n%w", kernelVersion, err)
	}

	ukiFullPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, fmt.Sprintf("%s.efi", kernel))

	ukifyCmd := []string{
		"-c", mainUkiConfigPath, "build",
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", ukiFullPath),
	}

	err = shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to build main UKI for config (%s):\n%w", mainUkiConfigPath, err)
	}

	logger.Log.Infof("Successfully built main UKI: (%s)", ukiFullPath)
	return nil
}

func buildUkiAddon(kernel string, kernelArgs string, stubPath string, systemBootPartitionTmpDir string) error {
	// Create addon directory: <uki-path>.extra.d/
	ukiFileName := fmt.Sprintf("%s.efi", kernel)
	ukiFullPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, ukiFileName)
	addonDirPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, fmt.Sprintf("%s.extra.d", ukiFileName))

	err := os.MkdirAll(addonDirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create addon directory (%s):\n%w", addonDirPath, err)
	}

	// Addon output path: <uki-name>.extra.d/<kernel-name>.addon.efi
	addonFullPath := filepath.Join(addonDirPath, fmt.Sprintf("%s.addon.efi", kernel))

	// Build the addon.
	ukifyCmd := []string{
		"build",
		fmt.Sprintf("--cmdline=%s", kernelArgs),
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", addonFullPath),
	}

	err = shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to build UKI addon:\n%w", err)
	}

	logger.Log.Infof("Successfully built UKI addon: (%s)", addonFullPath)
	logger.Log.Infof("Main UKI (%s) will load addon cmdline at boot time", ukiFullPath)
	return nil
}

func cleanupUkiBuildDir(buildDir string) error {
	ukiBuildDirPath := filepath.Join(buildDir, UkiBuildDir)

	err := os.RemoveAll(ukiBuildDirPath)
	if err != nil {
		return fmt.Errorf("failed to clean up UkiBuildDir at (%s):\n%w", ukiBuildDirPath, err)
	}

	logger.Log.Infof("Successfully cleaned up UkiBuildDir: (%s)", ukiBuildDirPath)
	return nil
}

func getKernelVersion(kernelName string) (string, error) {
	if !strings.HasPrefix(kernelName, KernelPrefix) {
		return "", fmt.Errorf("invalid kernel name: (%s), expected to start with prefix: (%s)", kernelName, KernelPrefix)
	}

	kernelVersion := strings.TrimPrefix(kernelName, "vmlinuz-")
	return kernelVersion, nil
}

func readUkiKernelInfoFile(filePath string) (map[string]UkiKernelInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kernel info file at (%s):\n%w", filePath, err)
	}

	var kernelInfo map[string]UkiKernelInfo
	err = json.Unmarshal(content, &kernelInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel info file at (%s):\n%w", filePath, err)
	}

	return kernelInfo, nil
}

func writeUkiKernelInfoFile(filePath string, kernelInfo map[string]UkiKernelInfo) error {
	content, err := json.MarshalIndent(kernelInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize kernel info to JSON:\n%w", err)
	}

	err = os.WriteFile(filePath, content, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write kernel info file at (%s):\n%w", filePath, err)
	}

	return nil
}

func getKernelNameFromUki(ukiPath string) (string, error) {
	fileName := filepath.Base(ukiPath)

	matches := ukiNamePattern.FindStringSubmatch(fileName)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid UKI file name: (%s)", fileName)
	}

	// Reconstruct kernel name (vmlinuz-<version>, e.g., vmlinuz-6.6.51.1-5.azl3)
	kernelName := "vmlinuz-" + matches[1]
	return kernelName, nil
}

func extractSectionFromUkiWithObjcopy(ukiPath string, sectionName string, outputPath string, buildDir string) error {
	tempCopy, err := os.CreateTemp(buildDir, "uki-copy-*.efi")
	if err != nil {
		return fmt.Errorf("failed to create temp UKI copy:\n%w", err)
	}
	defer os.Remove(tempCopy.Name())

	input, err := os.ReadFile(ukiPath)
	if err != nil {
		return fmt.Errorf("failed to read UKI file:\n%w", err)
	}
	if err := os.WriteFile(tempCopy.Name(), input, 0o644); err != nil {
		return fmt.Errorf("failed to write temp UKI file:\n%w", err)
	}

	// Extract the section using objcopy on the temp copy
	_, _, err = shell.Execute("objcopy", "--dump-section", sectionName+"="+outputPath, tempCopy.Name())
	if err != nil {
		return fmt.Errorf("objcopy failed to extract section %s:\n%w", sectionName, err)
	}

	return nil
}

func extractKernelAndInitramfsFromUkis(ctx context.Context, imageChroot *safechroot.Chroot, buildDir string) error {
	err := extractKernelAndInitramfsFromUkisHelper(ctx, imageChroot, buildDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIExtractComponents, err)
	}

	return nil
}

func extractKernelAndInitramfsFromUkisHelper(ctx context.Context, imageChroot *safechroot.Chroot, buildDir string) error {
	logger.Log.Infof("Extracting kernel and initramfs from existing UKIs for re-customization")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "extract_kernel_initramfs_from_ukis")
	defer span.End()

	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	ukiFiles, err := getUkiFiles(espDir)
	if err != nil {
		return err
	}

	if len(ukiFiles) == 0 {
		logger.Log.Infof("No existing UKI files found, skipping extraction")
		return nil
	}

	bootDir := filepath.Join(imageChroot.RootDir(), BootDir)

	tempDir := filepath.Join(buildDir, "uki-extraction-temp")
	err = os.MkdirAll(tempDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create temp directory:\n%w", err)
	}
	defer os.RemoveAll(tempDir)

	for _, ukiFile := range ukiFiles {
		kernelName, err := getKernelNameFromUki(ukiFile)
		if err != nil {
			return err
		}

		kernelVersion, err := getKernelVersion(kernelName)
		if err != nil {
			return err
		}

		// Extract kernel from main UKI file
		kernelPath := filepath.Join(bootDir, kernelName)
		logger.Log.Infof("Extracting kernel from main UKI (%s) to (%s)", ukiFile, kernelPath)
		err = extractSectionFromUkiWithObjcopy(ukiFile, ".linux", kernelPath, tempDir)
		if err != nil {
			return fmt.Errorf("failed to extract kernel from UKI (%s):\n%w", ukiFile, err)
		}

		initramfsName := fmt.Sprintf("initramfs-%s.img", kernelVersion)
		initramfsPath := filepath.Join(bootDir, initramfsName)

		logger.Log.Infof("Extracting initramfs from main UKI (%s) to (%s)", ukiFile, initramfsPath)
		err = extractSectionFromUkiWithObjcopy(ukiFile, ".initrd", initramfsPath, tempDir)
		if err != nil {
			return fmt.Errorf("failed to extract initramfs from UKI (%s):\n%w", ukiFile, err)
		}

		logger.Log.Infof("Successfully extracted kernel and initramfs for version (%s)", kernelVersion)
	}

	// Regenerate grub.cfg now that kernels are in /boot
	logger.Log.Infof("Regenerating grub.cfg after kernel extraction")

	// Ensure /boot/grub2 directory exists
	grubDir := filepath.Join(imageChroot.RootDir(), filepath.Dir(installutils.GrubCfgFile))
	err = os.MkdirAll(grubDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create grub directory (%s):\n%w", grubDir, err)
	}

	err = installutils.CallGrubMkconfig(imageChroot)
	if err != nil {
		return fmt.Errorf("failed to regenerate grub.cfg after kernel extraction:\n%w", err)
	}

	return nil
}

func cleanUkiDirectory(ukiOutputDir string) error {
	if _, err := os.Stat(ukiOutputDir); errors.Is(err, fs.ErrNotExist) {
		logger.Log.Debugf("UKI output directory does not exist, nothing to clean: (%s)", ukiOutputDir)
		return nil
	}

	entries, err := os.ReadDir(ukiOutputDir)
	if err != nil {
		return fmt.Errorf("failed to read UKI output directory (%s):\n%w", ukiOutputDir, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(ukiOutputDir, entry.Name())

		if entry.IsDir() {
			// Clean addon directories (*.efi.extra.d/)
			if strings.HasSuffix(entry.Name(), ".efi.extra.d") {
				err := os.RemoveAll(entryPath)
				if err != nil {
					return fmt.Errorf("failed to delete old UKI addon directory (%s):\n%w", entryPath, err)
				}
				logger.Log.Infof("Deleted old UKI addon directory: (%s)", entryPath)
			}
			continue
		}

		// Clean UKI files
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".efi") {
			err := os.Remove(entryPath)
			if err != nil {
				return fmt.Errorf("failed to delete old UKI file (%s):\n%w", entryPath, err)
			}
			logger.Log.Infof("Deleted old UKI file: (%s)", entryPath)
		}
	}

	return nil
}

func cleanBootDirectory(imageChroot *safechroot.Chroot) error {
	bootPath := filepath.Join(imageChroot.RootDir(), BootDir)
	espPath := filepath.Join(imageChroot.RootDir(), EspDir)

	dirEntries, err := os.ReadDir(bootPath)
	if err != nil {
		return fmt.Errorf("failed to read boot directory (%s):\n%w", bootPath, err)
	}

	for _, entry := range dirEntries {
		entryPath := filepath.Join(bootPath, entry.Name())

		if entryPath == espPath || entry.Name() == "lost+found" {
			continue
		}

		err := os.RemoveAll(entryPath)
		if err != nil {
			return fmt.Errorf("failed to remove (%s):\n%w", entryPath, err)
		}
	}

	return nil
}
