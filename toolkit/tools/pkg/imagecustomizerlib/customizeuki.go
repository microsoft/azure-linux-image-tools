// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"gopkg.in/ini.v1"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
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
)

const (
	BootDir            = "boot"
	EspDir             = "boot/efi"
	DefaultGrubCfgPath = "grub2/grub.cfg"
	UkiKernelInfoJson  = "uki-kernel-info.json"
	KernelPrefix       = "vmlinuz-"
	UkiBuildDir        = "UkiBuildDir"
	UkiOutputDir       = "EFI/Linux"
)

// Matches UKI filenames like "vmlinuz-<version>.efi"
var ukiNamePattern = regexp.MustCompile(`^vmlinuz-(.+)\.efi$`)

// UkiKernelInfo holds both command line arguments and initramfs name for a UKI kernel
type UkiKernelInfo struct {
	Cmdline   string `json:"cmdline"`
	Initramfs string `json:"initramfs"`
}

func prepareUki(ctx context.Context, buildDir string, uki *imagecustomizerapi.Uki, imageChroot *safechroot.Chroot,
	distroHandler distroHandler,
) error {
	err := prepareUkiHelper(ctx, buildDir, uki, imageChroot, distroHandler)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIPrepareOS, err)
	}

	return nil
}

func prepareUkiHelper(ctx context.Context, buildDir string, uki *imagecustomizerapi.Uki, imageChroot *safechroot.Chroot,
	distroHandler distroHandler,
) error {
	var err error

	if uki == nil {
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
	kernelToInitramfs, err := getKernelToInitramfsMap(bootDir, uki.Kernels)
	if err != nil {
		return fmt.Errorf("%w (bootDir='%s'):\n%w", ErrUKIKernelInitramfsMap, bootDir, err)
	}

	// Copy UKI-specific files such as kernel, initramfs, and UKI stub file.
	err = copyUkiFiles(buildDir, kernelToInitramfs, imageChroot, bootConfig)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIFileCopy, err)
	}

	// Extract kernel command line arguments from either grub.cfg or UKI.
	espDir := filepath.Join(imageChroot.RootDir(), EspDir)
	kernelToArgs, err := extractKernelToArgs(espDir, bootDir, buildDir)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUKIKernelCmdlineExtract, err)
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
	bootConfig BootFilesArchConfig,
) error {
	filesToCopy := map[string]string{
		filepath.Join(imageChroot.RootDir(), bootConfig.ukiEfiStubBinaryPath): filepath.Join(buildDir, UkiBuildDir, bootConfig.ukiEfiStubBinary),
		filepath.Join(imageChroot.RootDir(), "/etc/os-release"):               filepath.Join(buildDir, UkiBuildDir, "os-release"),
	}

	for kernel, initramfs := range kernelToInitramfs {
		kernelSource := filepath.Join(imageChroot.RootDir(), BootDir, kernel)
		kernelDest := filepath.Join(buildDir, UkiBuildDir, kernel)
		filesToCopy[kernelSource] = kernelDest

		initramfsSource := filepath.Join(imageChroot.RootDir(), BootDir, initramfs)
		initramfsDest := filepath.Join(buildDir, UkiBuildDir, initramfs)
		filesToCopy[initramfsSource] = initramfsDest
	}

	for src, dest := range filesToCopy {
		err := file.Copy(src, dest)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrUKIFileCopy, err)
		}
	}

	return nil
}

func getKernelToInitramfsMap(bootDir string, ukiKernels imagecustomizerapi.UkiKernels) (map[string]string, error) {
	if ukiKernels.Auto {
		// Auto mode: Find all kernels and their initramfs.
		kernelToInitramfs, err := findKernelsAndInitramfs(bootDir)
		if err != nil {
			return nil, fmt.Errorf("failed to find kernels and initramfs in auto mode:\n%w", err)
		}
		return kernelToInitramfs, nil
	}

	// User-specified mode: Match kernels and initramfs with the specified versions.
	kernelToInitramfs, err := findSpecificKernelsAndInitramfs(bootDir, ukiKernels.Kernels)
	if err != nil {
		return nil, fmt.Errorf("failed to find specific kernels and initramfs:\n%w", err)
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

func findSpecificKernelsAndInitramfs(bootDir string, versions []string) (map[string]string, error) {
	kernelToInitramfs := make(map[string]string)

	for _, version := range versions {
		kernelName := fmt.Sprintf("vmlinuz-%s", version)
		initramfsName := fmt.Sprintf("initramfs-%s.img", version)

		kernelPath := filepath.Join(bootDir, kernelName)
		initramfsPath := filepath.Join(bootDir, initramfsName)

		kernelExists, err := file.PathExists(kernelPath)
		if err != nil {
			return nil, fmt.Errorf("error checking existence of kernel (%s):\n%w", kernelPath, err)
		}
		if !kernelExists {
			return nil, fmt.Errorf("missing kernel: (%s)", kernelName)
		}

		initramfsExists, err := file.PathExists(initramfsPath)
		if err != nil {
			return nil, fmt.Errorf("error checking existence of initramfs (%s):\n%w", initramfsPath, err)
		}
		if !initramfsExists {
			return nil, fmt.Errorf("missing initramfs for kernel: (%s), expected (%s)", kernelName, initramfsName)
		}

		kernelToInitramfs[kernelName] = initramfsName
	}

	return kernelToInitramfs, nil
}

func createUki(ctx context.Context, buildDir string, buildImageFile string) error {
	logger.Log.Infof("Creating UKIs")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "customize_uki")
	defer span.End()

	var err error

	_, bootConfig, err := getBootArchConfig()
	if err != nil {
		return err
	}

	loopback, err := safeloopback.NewLoopback(buildImageFile)
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

	systemBootPartitionTmpDir := filepath.Join(buildDir, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path, systemBootPartitionTmpDir, systemBootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", systemBootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	stubPath := filepath.Join(buildDir, UkiBuildDir, bootConfig.ukiEfiStubBinary)
	osSubreleaseFullPath := filepath.Join(buildDir, UkiBuildDir, "os-release")
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)

	// Read the kernel information (kernels, initramfs, and command line args) from the file created during prepareUki.
	kernelInfo, err := readUkiKernelInfoFile(cmdlineFilePath)
	if err != nil {
		return err
	}

	for kernel, info := range kernelInfo {
		err := buildUki(kernel, info.Initramfs, info.Cmdline, osSubreleaseFullPath, stubPath, buildDir,
			systemBootPartitionTmpDir,
		)
		if err != nil {
			return fmt.Errorf("failed to build UKI for kernel (%s):\n%w", kernel, err)
		}
	}

	err = cleanupUkiBuildDir(buildDir)
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
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to extract kernel args from grub.cfg:\n%w", err)
	} else if !os.IsNotExist(err) {
		return kernelToArgs, nil
	}

	// Fallback to extracting from UKI
	kernelToArgs, err = extractKernelCmdlineFromUkiEfis(espPath, buildDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to extract kernel args from UKI:\n%w", err)
	} else if os.IsNotExist(err) {
		return nil, fmt.Errorf("no kernel arguments found from either grub.cfg or UKI")
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
		// Normalize kernel path: strip "boot/" prefix if present. When there's
		// no separate /boot partition, grub.cfg has paths like
		// "boot/vmlinuz-*" but kernel discovery returns just "vmlinuz-*".
		normalizedKernel := strings.TrimPrefix(kernel, "boot/")

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
	configFilePath := filepath.Join(buildDir, UkiBuildDir, fmt.Sprintf("ukify_%s.conf", kernelVersion))

	// Create the INI file.
	cfg := ini.Empty()
	section, err := cfg.NewSection("UKI")
	if err != nil {
		return fmt.Errorf("failed to create INI section:\n%w", err)
	}

	// Add keys to the INI file.
	_, err = section.NewKey("Linux", filepath.Join(buildDir, UkiBuildDir, kernel))
	if err != nil {
		return fmt.Errorf("failed to add 'Linux' key to INI file:\n%w", err)
	}

	_, err = section.NewKey("Initrd", filepath.Join(buildDir, UkiBuildDir, initramfs))
	if err != nil {
		return fmt.Errorf("failed to add 'Initrd' key to INI file:\n%w", err)
	}

	_, err = section.NewKey("Cmdline", kernelArgs)
	if err != nil {
		return fmt.Errorf("failed to add 'Cmdline' key to INI file:\n%w", err)
	}

	_, err = section.NewKey("OSRelease", fmt.Sprintf("@%s", osSubreleaseFullPath))
	if err != nil {
		return fmt.Errorf("failed to add 'OSRelease' key to INI file:\n%w", err)
	}

	// Save the INI file.
	err = cfg.SaveTo(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to save INI file for kernel (%s):\n%w", kernelVersion, err)
	}

	ukiFullPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, fmt.Sprintf("%s.efi", kernel))

	// Build the UKI using ukify.
	ukifyCmd := []string{
		"-c", configFilePath, "build",
		fmt.Sprintf("--stub=%s", stubPath),
		fmt.Sprintf("--output=%s", ukiFullPath),
	}

	err = shell.ExecuteLiveWithErr(1, "ukify", ukifyCmd...)
	if err != nil {
		return fmt.Errorf("failed to build UKI for config (%s):\n%w", configFilePath, err)
	}

	logger.Log.Infof("Successfully built UKI: (%s)", ukiFullPath)
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

func appendKernelArgsToUkiCmdlineFile(buildDir string, newArgs []string) error {
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, UkiKernelInfoJson)

	kernelInfo, err := readUkiKernelInfoFile(cmdlineFilePath)
	if err != nil {
		return err
	}

	// Append newArgs.
	newArgsStr := GrubArgsToString(newArgs)
	for kernel, info := range kernelInfo {
		updatedArgs := fmt.Sprintf("%s %s", strings.TrimSpace(info.Cmdline), strings.TrimSpace(newArgsStr))
		kernelInfo[kernel] = UkiKernelInfo{
			Cmdline:   updatedArgs,
			Initramfs: info.Initramfs,
		}
	}

	err = writeUkiKernelInfoFile(cmdlineFilePath, kernelInfo)
	if err != nil {
		return err
	}

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
