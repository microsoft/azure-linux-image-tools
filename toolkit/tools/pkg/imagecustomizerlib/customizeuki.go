// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"encoding/json"
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

const (
	BootDir               = "boot"
	DefaultGrubCfgPath    = "grub2/grub.cfg"
	KernelCmdlineArgsJson = "kernel-cmdline-args.json"
	KernelPrefix          = "vmlinuz-"
	UkiBuildDir           = "UkiBuildDir"
	UkiOutputDir          = "EFI/Linux"
)

func prepareUki(buildDir string, uki *imagecustomizerapi.Uki, imageChroot *safechroot.Chroot) error {
	var err error

	if uki == nil {
		return nil
	}

	logger.Log.Infof("Enabling UKI")

	// Check UKI dependency packages.
	err = validateUkiDependencies(imageChroot)
	if err != nil {
		return fmt.Errorf("failed to validate package dependencies for uki:\n%w", err)
	}

	// Create necessary directories for UKI.
	err = createUkiDirectories(buildDir, imageChroot)
	if err != nil {
		return fmt.Errorf("failed to create UKI directories:\n%w", err)
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
		return fmt.Errorf("failed to install systemd-boot:\n%w", err)
	}

	// The "--random-seed=no" flag is preferred to disable this behavior, but it requires systemd version 257 or later.
	// Since AZL 3.0 uses version 255, we manually remove the random-seed file here for now.
	randomSeedPath := filepath.Join(imageChroot.RootDir(), "/boot/efi/loader/random-seed")
	if err := file.RemoveFileIfExists(randomSeedPath); err != nil {
		return fmt.Errorf("failed to remove random-seed file (%s):\n%w", randomSeedPath, err)
	}

	// Map kernels and initramfs.
	bootDir := filepath.Join(imageChroot.RootDir(), BootDir)
	kernelToInitramfs, err := getKernelToInitramfsMap(bootDir, uki.Kernels)
	if err != nil {
		return fmt.Errorf("failed to get kernel to initramfs map:\n%w", err)
	}

	// Copy UKI-specific files such as kernel, initramfs, and UKI stub file.
	err = copyUkiFiles(buildDir, kernelToInitramfs, imageChroot)
	if err != nil {
		return fmt.Errorf("failed to copy UKI files:\n%w", err)
	}

	// Extract kernel command line arguments from grub.cfg.
	grubCfgPath := filepath.Join(bootDir, DefaultGrubCfgPath)
	kernelToArgs, err := extractKernelToArgsFromGrub(grubCfgPath)
	if err != nil {
		return fmt.Errorf("failed to extract kernel command-line arguments:\n%w", err)
	}

	// Dump kernel command line arguments to a file in buildDir.
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, KernelCmdlineArgsJson)
	err = writeKernelCmdlineArgsFile(cmdlineFilePath, kernelToArgs)
	if err != nil {
		return fmt.Errorf("failed to write kernel cmdline args JSON to (%s):\n%w", cmdlineFilePath, err)
	}

	return nil
}

func validateUkiDependencies(imageChroot *safechroot.Chroot) error {
	// The following packages are required for the UKI feature:
	// - "systemd-ukify": Required to build the Unified Kernel Image.
	// - "systemd-boot": Checked as a package dependency here to ensure installation,
	//    but additional configuration is handled elsewhere in the UKI workflow.
	// - "efibootmgr": Used for managing EFI boot entries.
	requiredRpms := []string{"systemd-ukify", "systemd-boot", "efibootmgr"}

	// Iterate over each required package and check if it's installed.
	for _, pkg := range requiredRpms {
		logger.Log.Debugf("Checking if package (%s) is installed", pkg)
		if !isPackageInstalled(imageChroot, pkg) {
			return fmt.Errorf("package (%s) is not installed:\nthe following packages must be installed to use Uki: (%v)", pkg, requiredRpms)
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

func copyUkiFiles(buildDir string, kernelToInitramfs map[string]string, imageChroot *safechroot.Chroot) error {
	filesToCopy := map[string]string{
		filepath.Join(imageChroot.RootDir(), "/usr/lib/systemd/boot/efi/linuxx64.efi.stub"): filepath.Join(buildDir, UkiBuildDir, "linuxx64.efi.stub"),
		filepath.Join(imageChroot.RootDir(), "/etc/os-release"):                             filepath.Join(buildDir, UkiBuildDir, "os-release"),
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
			return fmt.Errorf("failed to copy file from (%s) to (%s):\n%w", src, dest, err)
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

func createUki(uki *imagecustomizerapi.Uki, buildDir string, buildImageFile string) error {
	logger.Log.Debugf("Customizing UKI")

	var err error

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
	bootPartition, err := findBootPartitionFromEsp(systemBootPartition, diskPartitions, buildDir)
	if err != nil {
		return err
	}

	systemBootPartitionTmpDir := filepath.Join(buildDir, tmpEspPartitionDirName)
	systemBootPartitionMount, err := safemount.NewMount(systemBootPartition.Path, systemBootPartitionTmpDir, systemBootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount esp partition (%s):\n%w", bootPartition.Path, err)
	}
	defer systemBootPartitionMount.Close()

	bootPartitionTmpDir := filepath.Join(buildDir, tmpParitionDirName)
	bootPartitionMount, err := safemount.NewMount(bootPartition.Path, bootPartitionTmpDir, bootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("failed to mount partition (%s):\n%w", bootPartition.Path, err)
	}
	defer bootPartitionMount.Close()

	osSubreleaseFullPath := filepath.Join(buildDir, UkiBuildDir, "os-release")
	stubPath := filepath.Join(buildDir, UkiBuildDir, "linuxx64.efi.stub")
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, KernelCmdlineArgsJson)

	// Get mapped kernels and initramfs.
	kernelToInitramfs, err := getKernelToInitramfsMap(bootPartitionTmpDir, uki.Kernels)
	if err != nil {
		return err
	}

	for kernel, initramfs := range kernelToInitramfs {
		err := buildUki(kernel, initramfs, cmdlineFilePath, osSubreleaseFullPath, stubPath, buildDir,
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

	err = cleanupBootPartition(bootPartitionTmpDir)
	if err != nil {
		return fmt.Errorf("failed to clean up boot partition:\n%w", err)
	}

	err = systemBootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	err = bootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	err = loopback.CleanClose()
	if err != nil {
		return err
	}

	return nil
}

// Note: This function will be optimized by leveraging the internal functions
// under grubcfgutils.go when implementing bootloader customization.
func extractKernelToArgsFromGrub(grubCfgPath string) (map[string]string, error) {
	grubCfgContent, err := file.Read(grubCfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read grub.cfg file at (%s):\n%w", grubCfgPath, err)
	}

	linuxLineRegex := regexp.MustCompile(`^linux\s+(/vmlinuz-[^\s]+)\s+(.*)`)

	lines := strings.Split(string(grubCfgContent), "\n")
	kernelToArgs := make(map[string]string)

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		matches := linuxLineRegex.FindStringSubmatch(trimmedLine)
		if len(matches) == 3 {
			kernel := strings.TrimPrefix(matches[1], "/")
			args := matches[2]
			kernelToArgs[kernel] = strings.TrimSpace(args)
		}
	}

	if len(kernelToArgs) == 0 {
		return nil, fmt.Errorf("failed to find any valid 'linux /vmlinuz-*' lines in grub.cfg file at (%s)", grubCfgPath)
	}

	return kernelToArgs, nil
}

func buildUki(kernel string, initramfs string, cmdlineFilePath string, osSubreleaseFullPath string,
	stubPath string, buildDir string, systemBootPartitionTmpDir string,
) error {
	kernelVersion, err := getKernelVersion(kernel)
	if err != nil {
		return err
	}
	configFilePath := filepath.Join(buildDir, UkiBuildDir, fmt.Sprintf("ukify_%s.conf", kernelVersion))

	kernelToArgs, err := readKernelCmdlineArgsFile(cmdlineFilePath)
	if err != nil {
		return err
	}

	// Get the arguments for the current kernel.
	kernelArgs, ok := kernelToArgs[kernel]
	if !ok {
		return fmt.Errorf("no kernel cmdline arguments found for kernel (%s)", kernel)
	}

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

	ukiFullPath := filepath.Join(systemBootPartitionTmpDir, UkiOutputDir, fmt.Sprintf("%s.unsigned.efi", kernel))

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

func cleanupBootPartition(bootPartitionTmpDir string) error {
	dirEntries, err := os.ReadDir(bootPartitionTmpDir)
	if err != nil {
		return fmt.Errorf("failed to read boot partition directory (%s):\n%w", bootPartitionTmpDir, err)
	}

	for _, entry := range dirEntries {
		entryPath := filepath.Join(bootPartitionTmpDir, entry.Name())

		err := os.RemoveAll(entryPath)
		if err != nil {
			return fmt.Errorf("failed to remove (%s):\n%w", entryPath, err)
		}
	}

	logger.Log.Infof("Successfully cleaned up boot partition: (%s)", bootPartitionTmpDir)
	return nil
}

func appendKernelArgsToUkiCmdlineFile(buildDir string, newArgs []string) error {
	cmdlineFilePath := filepath.Join(buildDir, UkiBuildDir, KernelCmdlineArgsJson)

	kernelToArgs, err := readKernelCmdlineArgsFile(cmdlineFilePath)
	if err != nil {
		return err
	}

	// Append newArgs.
	newArgsStr := strings.Join(newArgs, " ")
	for kernel, args := range kernelToArgs {
		updatedArgs := fmt.Sprintf("%s %s", strings.TrimSpace(args), strings.TrimSpace(newArgsStr))
		kernelToArgs[kernel] = updatedArgs
	}

	err = writeKernelCmdlineArgsFile(cmdlineFilePath, kernelToArgs)
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

func readKernelCmdlineArgsFile(filePath string) (map[string]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kernel cmdline args file at (%s):\n%w", filePath, err)
	}

	var kernelToArgs map[string]string
	err = json.Unmarshal(content, &kernelToArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel cmdline args file at (%s):\n%w", filePath, err)
	}

	return kernelToArgs, nil
}

func writeKernelCmdlineArgsFile(filePath string, kernelToArgs map[string]string) error {
	content, err := json.MarshalIndent(kernelToArgs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize kernel cmdline args to JSON:\n%w", err)
	}

	err = os.WriteFile(filePath, content, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write kernel cmdline args file at (%s):\n%w", filePath, err)
	}

	return nil
}
