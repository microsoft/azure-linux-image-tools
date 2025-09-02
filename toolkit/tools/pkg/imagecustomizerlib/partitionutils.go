// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

var (
	bootPartitionRegex = regexp.MustCompile(`(?m)^search -n -u ([a-zA-Z0-9\-]+) -s$`)

	// Extract the partition number from the loopback partition path.
	partitionNumberRegex = regexp.MustCompile(`^/dev/loop\d+p(\d+)$`)
)

func findSystemBootPartition(diskPartitions []diskutils.PartitionInfo) (*diskutils.PartitionInfo, error) {
	// Look for all system boot partitions, including both EFI System Paritions (ESP) and BIOS boot partitions.
	var bootPartitions []*diskutils.PartitionInfo
	for i := range diskPartitions {
		diskPartition := diskPartitions[i]

		switch diskPartition.PartitionTypeUuid {
		case diskutils.EfiSystemPartitionTypeUuid, diskutils.BiosBootPartitionTypeUuid:
			bootPartitions = append(bootPartitions, &diskPartition)
		}
	}

	if len(bootPartitions) > 1 {
		return nil, fmt.Errorf("found more than one boot partition (ESP or BIOS boot parititon)")
	} else if len(bootPartitions) < 1 {
		return nil, fmt.Errorf("failed to find boot partition (ESP or BIOS boot parititon)")
	}

	bootPartition := bootPartitions[0]
	return bootPartition, nil
}

func findBootPartitionFromEsp(efiSystemPartition *diskutils.PartitionInfo, diskPartitions []diskutils.PartitionInfo, buildDir string) (*diskutils.PartitionInfo, error) {
	tmpDir := filepath.Join(buildDir, tmpEspPartitionDirName)

	// Mount the EFI System Partition.
	efiSystemPartitionMount, err := safemount.NewMount(efiSystemPartition.Path, tmpDir, efiSystemPartition.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount EFI system partition:\n%w", err)
	}
	defer efiSystemPartitionMount.Close()

	// Read the grub.cfg file.
	grubConfigFilePath := filepath.Join(tmpDir, installutils.GrubCfgFile)
	grubConfigFile, err := os.ReadFile(grubConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read EFI system partition's grub.cfg file:\n%w", err)
	}

	// Close the EFI System Partition mount.
	err = efiSystemPartitionMount.CleanClose()
	if err != nil {
		return nil, fmt.Errorf("failed to close EFI system partition mount:\n%w", err)
	}

	// Look for the bootloader partition declaration line in the grub.cfg file.
	match := bootPartitionRegex.FindStringSubmatch(string(grubConfigFile))
	if match == nil {
		return nil, fmt.Errorf("failed to find boot partition in grub.cfg file")
	}

	bootPartitionUuid := match[1]

	var bootPartition *diskutils.PartitionInfo
	for i := range diskPartitions {
		diskPartition := diskPartitions[i]

		if diskPartition.Uuid == bootPartitionUuid {
			bootPartition = &diskPartition
			break
		}
	}

	if bootPartition == nil {
		return nil, fmt.Errorf("failed to find boot partition with UUID (%s)", bootPartitionUuid)
	}

	return bootPartition, nil
}

// Searches for the partition that contains the /etc/fstab file.
// While technically it is possible to place /etc on a different partition, doing so is fairly difficult and requires
// a custom initramfs module.
func findRootfsPartition(diskPartitions []diskutils.PartitionInfo, buildDir string) (*diskutils.PartitionInfo, error) {
	logger.Log.Debugf("Searching for rootfs partition")

	tmpDir := filepath.Join(buildDir, tmpPartitionDirName)

	var rootfsPartitions []*diskutils.PartitionInfo
	for i := range diskPartitions {
		diskPartition := diskPartitions[i]

		// Skip over disk entries.
		if diskPartition.Type != "part" {
			continue
		}

		// Skip over file-system types that can't be used for the rootfs partition.
		switch diskPartition.FileSystemType {
		case "ext2", "ext3", "ext4", "xfs":

		default:
			logger.Log.Debugf("Skip partition (%s) with unsupported rootfs filesystem type (%s)", diskPartition.Path,
				diskPartition.FileSystemType)
			continue
		}

		// Temporarily mount the partition.
		partitionMount, err := safemount.NewMount(diskPartition.Path, tmpDir, diskPartition.FileSystemType, unix.MS_RDONLY,
			"", true)
		if err != nil {
			return nil, fmt.Errorf("failed to mount partition (%s):\n%w", diskPartition.Path, err)
		}
		defer partitionMount.Close()

		// Check if the /etc/fstab file exists.
		fstabPath := filepath.Join(tmpDir, "/etc/fstab")
		exists, err := file.PathExists(fstabPath)
		if err != nil {
			return nil, fmt.Errorf("failed to check if /etc/fstab file exists (%s):\n%w", diskPartition.Path, err)
		}

		if exists {
			rootfsPartitions = append(rootfsPartitions, &diskPartition)
		}

		// Close the rootfs partition mount.
		err = partitionMount.CleanClose()
		if err != nil {
			return nil, fmt.Errorf("failed to close partition mount (%s):\n%w", diskPartition.Path, err)
		}
	}

	if len(rootfsPartitions) > 1 {
		return nil, fmt.Errorf("found too many rootfs partition candidates (%d)", len(rootfsPartitions))
	} else if len(rootfsPartitions) < 1 {
		return nil, fmt.Errorf("failed to find rootfs partition")
	}

	rootfsPartition := rootfsPartitions[0]
	return rootfsPartition, nil
}

func readFstabEntriesFromRootfs(rootfsPartition *diskutils.PartitionInfo, diskPartitions []diskutils.PartitionInfo,
	buildDir string,
) ([]diskutils.FstabEntry, error) {
	logger.Log.Debugf("Reading fstab entries")

	tmpDir := filepath.Join(buildDir, tmpPartitionDirName)

	// Temporarily mount the rootfs partition so that the fstab file can be read.
	rootfsPartitionMount, err := safemount.NewMount(rootfsPartition.Path, tmpDir, rootfsPartition.FileSystemType, unix.MS_RDONLY, "",
		true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount rootfs partition (%s):\n%w", rootfsPartition.Path, err)
	}
	defer rootfsPartitionMount.Close()

	// Read the fstab file.
	fstabPath := filepath.Join(tmpDir, "/etc/fstab")

	// Read the fstab file.
	fstabEntries, err := diskutils.ReadFstabFile(fstabPath)
	if err != nil {
		return nil, err
	}

	// Close the rootfs partition mount.
	err = rootfsPartitionMount.CleanClose()
	if err != nil {
		return nil, fmt.Errorf("failed to close rootfs partition mount (%s):\n%w", rootfsPartition.Path, err)
	}

	return fstabEntries, nil
}

func fstabEntriesToMountPoints(fstabEntries []diskutils.FstabEntry, diskPartitions []diskutils.PartitionInfo,
	buildDir string, readonly bool, readOnlyVerity bool, ignoreOverlays bool,
) ([]*safechroot.MountPoint, map[string]diskutils.FstabEntry, []verityDeviceMetadata, []string, error) {
	filteredFstabEntries := filterOutSpecialPartitions(fstabEntries)

	// Convert fstab entries into mount points.
	var mountPoints []*safechroot.MountPoint
	var foundRoot bool
	partUuidToFstabEntry := make(map[string]diskutils.FstabEntry)
	verityMetadataList := []verityDeviceMetadata(nil)
	readonlyPartUuids := []string(nil)
	for _, fstabEntry := range filteredFstabEntries {
		if ignoreOverlays && strings.ToLower(fstabEntry.FsType) == "overlay" {
			continue // Skip overlay entries when requested
		}
		_, partition, _, verityMetadata, err := findSourcePartition(fstabEntry.Source, diskPartitions, buildDir)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		readOnlyPartition := readOnlyVerity && verityMetadata != nil
		if readOnlyPartition {
			readonlyPartUuids = append(readonlyPartUuids, partition.PartUuid)
		}

		var vfsOptions diskutils.MountFlags
		if readonly || readOnlyPartition {
			// In scenarios where the image has completed customization (e.g. when the image is re-mounted for injection),
			// force read-only mount. Since we're only reading data, execution and write permissions aren't needed.
			vfsOptions = fstabEntry.VfsOptions | diskutils.MountFlags(unix.MS_RDONLY)
		} else {
			// Unset read-only flag so that read-only partitions can be customized.
			// Unset noexec flag so that if rootfs is set as noexec, image can still be customized. For example, allowing
			// grub2-mkconfig to be called.
			vfsOptions = fstabEntry.VfsOptions & ^diskutils.MountFlags(unix.MS_RDONLY|unix.MS_NOEXEC)
		}

		var mountPoint *safechroot.MountPoint
		if fstabEntry.Target == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(
				partition.Path, fstabEntry.Target, fstabEntry.FsType,
				uintptr(vfsOptions), fstabEntry.FsOptions)

			foundRoot = true
		} else {
			mountPoint = safechroot.NewMountPoint(
				partition.Path, fstabEntry.Target, fstabEntry.FsType,
				uintptr(vfsOptions), fstabEntry.FsOptions)
		}

		mountPoints = append(mountPoints, mountPoint)
		partUuidToFstabEntry[partition.PartUuid] = fstabEntry
		if verityMetadata != nil {
			verityMetadataList = append(verityMetadataList, *verityMetadata)
		}
	}

	if !foundRoot {
		return nil, nil, nil, nil, fmt.Errorf("image has invalid fstab file: no root partition found")
	}

	return mountPoints, partUuidToFstabEntry, verityMetadataList, readonlyPartUuids, nil
}

func filterOutSpecialPartitions(fstabEntries []diskutils.FstabEntry) []diskutils.FstabEntry {
	filteredFstabEntries := []diskutils.FstabEntry(nil)
	for _, fstabEntry := range fstabEntries {
		// Ignore special partitions.
		if isSpecialPartition(fstabEntry) {
			continue
		}
		filteredFstabEntries = append(filteredFstabEntries, fstabEntry)
	}
	return filteredFstabEntries
}

func isSpecialPartition(fstabEntry diskutils.FstabEntry) bool {
	switch fstabEntry.FsType {
	case "devtmpfs", "proc", "sysfs", "devpts", "tmpfs":
		return true

	default:
		return false
	}
}

func findSourcePartition(source string, partitions []diskutils.PartitionInfo,
	buildDir string,
) (ExtendedMountIdentifierType, diskutils.PartitionInfo, int, *verityDeviceMetadata, error) {
	mountIdType, mountId, err := parseExtendedSourcePartition(source)
	if err != nil {
		return ExtendedMountIdentifierTypeDefault, diskutils.PartitionInfo{}, 0, nil, err
	}

	partition, partitionIndex, verityMetadata, err := findExtendedPartition(mountIdType, mountId, partitions, buildDir)
	if err != nil {
		return ExtendedMountIdentifierTypeDefault, diskutils.PartitionInfo{}, 0, nil, err
	}

	return mountIdType, partition, partitionIndex, verityMetadata, nil
}

func findPartition(mountIdType imagecustomizerapi.MountIdentifierType, mountId string,
	partitions []diskutils.PartitionInfo, buildDir string,
) (diskutils.PartitionInfo, int, error) {
	partition, partitionIndex, err := findPartitionHelper(mountIdType, mountId, partitions)
	if err != nil {
		return diskutils.PartitionInfo{}, 0, err
	}

	return partition, partitionIndex, nil
}

// findExtendedPartition extends the public func findPartition to handle additional identifier types.
func findExtendedPartition(mountIdType ExtendedMountIdentifierType, mountId string,
	partitions []diskutils.PartitionInfo, buildDir string,
) (diskutils.PartitionInfo, int, *verityDeviceMetadata, error) {
	switch mountIdType {
	case ExtendedMountIdentifierTypeDev:
		partition, partitionIndex, verityMetadata, err := findDevPathPartition(mountId, partitions, buildDir)
		if err != nil {
			return diskutils.PartitionInfo{}, 0, nil, err
		}

		return partition, partitionIndex, verityMetadata, err

	default:
		partition, partitionIndex, err := findPartitionHelper(imagecustomizerapi.MountIdentifierType(mountIdType),
			mountId, partitions)
		if err != nil {
			return diskutils.PartitionInfo{}, 0, nil, err
		}

		return partition, partitionIndex, nil, err
	}
}

func findPartitionHelper(mountIdType imagecustomizerapi.MountIdentifierType, mountId string,
	partitions []diskutils.PartitionInfo,
) (diskutils.PartitionInfo, int, error) {
	matchedPartitionIndexes := []int(nil)
	for i, partition := range partitions {
		matches := false
		switch mountIdType {
		case imagecustomizerapi.MountIdentifierTypeUuid:
			matches = partition.Uuid == mountId
		case imagecustomizerapi.MountIdentifierTypePartUuid:
			matches = partition.PartUuid == mountId
		case imagecustomizerapi.MountIdentifierTypePartLabel:
			matches = partition.PartLabel == mountId
		}
		if matches {
			matchedPartitionIndexes = append(matchedPartitionIndexes, i)
		}
	}

	if len(matchedPartitionIndexes) < 1 {
		err := fmt.Errorf("partition not found (%s=%s)", mountIdType, mountId)
		return diskutils.PartitionInfo{}, 0, err
	}
	if len(matchedPartitionIndexes) > 1 {
		err := fmt.Errorf("too many matches for partition found (%s=%s)", mountIdType, mountId)
		return diskutils.PartitionInfo{}, 0, err
	}

	partitionIndex := matchedPartitionIndexes[0]
	partition := partitions[partitionIndex]

	return partition, partitionIndex, nil
}

func findDevPathPartition(mountId string, partitions []diskutils.PartitionInfo,
	buildDir string,
) (diskutils.PartitionInfo, int, *verityDeviceMetadata, error) {
	cmdline, err := extractKernelCmdline(partitions, buildDir)
	if err != nil {
		return diskutils.PartitionInfo{}, 0, nil, err
	}

	switch mountId {
	case imagecustomizerapi.VerityRootDevicePath:
		partition, partitionIndex, verityMetadata, err := findVerityPartitionsFromCmdline(partitions, cmdline,
			"systemd.verity_root_data", "systemd.verity_root_hash", "roothash", "systemd.verity_root_options",
			imagecustomizerapi.VerityRootDeviceName)
		if err != nil {
			err = fmt.Errorf("failed to find %s verity partitions:\n%w", imagecustomizerapi.VerityRootDeviceName, err)
			return diskutils.PartitionInfo{}, 0, nil, err
		}
		return partition, partitionIndex, &verityMetadata, nil

	case imagecustomizerapi.VerityUsrDevicePath:
		partition, partitionIndex, verityMetadata, err := findVerityPartitionsFromCmdline(partitions, cmdline,
			"systemd.verity_usr_data", "systemd.verity_usr_hash", "usrhash", "systemd.verity_usr_options",
			imagecustomizerapi.VerityUsrDeviceName)
		if err != nil {
			err = fmt.Errorf("failed to find %s verity partitions:\n%w", imagecustomizerapi.VerityUsrDeviceName, err)
			return diskutils.PartitionInfo{}, 0, nil, err
		}
		return partition, partitionIndex, &verityMetadata, nil

	default:
		err = fmt.Errorf("unknown partition id (%s)", mountId)
		return diskutils.PartitionInfo{}, 0, nil, err
	}
}

func findVerityPartitionsFromCmdline(partitions []diskutils.PartitionInfo, cmdline []grubConfigLinuxArg,
	dataDeviceArgName string, hashDeviceArgName string, hashArgName string, optionsArgName string, name string,
) (diskutils.PartitionInfo, int, verityDeviceMetadata, error) {
	dataIdType, dataId, err := extractVerityPartitionId(cmdline, dataDeviceArgName)
	if err != nil {
		err = fmt.Errorf("failed to parse '%s' kernel arg:\n%w", dataDeviceArgName, err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	hashIdType, hashId, err := extractVerityPartitionId(cmdline, hashDeviceArgName)
	if err != nil {
		err = fmt.Errorf("failed to parse '%s' kernel arg:\n%w", hashDeviceArgName, err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	dataPartition, dataPartitionIndex, err := findPartitionHelper(dataIdType, dataId, partitions)
	if err != nil {
		err = fmt.Errorf("failed to find verity data partition:\n%w", err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	hashPartition, _, err := findPartitionHelper(hashIdType, hashId, partitions)
	if err != nil {
		err = fmt.Errorf("failed to find verity data partition:\n%w", err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	hash, err := findKernelCommandLineArgValue(cmdline, hashArgName)
	if err != nil {
		err = fmt.Errorf("failed get verity hash (%s) kernel argument:\n%w", hashArgName, err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	options, err := findKernelCommandLineArgValue(cmdline, optionsArgName)
	if err != nil {
		err = fmt.Errorf("failed get verity options (%s) kernel argument:\n%w", optionsArgName, err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	corruptionOption, hashSigPath, err := parseSystemdVerityOptions(options)
	if err != nil {
		err = fmt.Errorf("failed parse verity options (%s) kernel argument:\n%w", optionsArgName, err)
		return diskutils.PartitionInfo{}, 0, verityDeviceMetadata{}, err
	}

	verityMetadata := verityDeviceMetadata{
		name:                  name,
		rootHash:              hash,
		dataPartUuid:          dataPartition.PartUuid,
		hashPartUuid:          hashPartition.PartUuid,
		dataDeviceMountIdType: dataIdType,
		hashDeviceMountIdType: hashIdType,
		corruptionOption:      corruptionOption,
		hashSignaturePath:     hashSigPath,
	}

	return dataPartition, dataPartitionIndex, verityMetadata, nil
}

func extractKernelCmdline(partitions []diskutils.PartitionInfo, buildDir string) ([]grubConfigLinuxArg, error) {
	espPartition, err := findSystemBootPartition(partitions)
	if err != nil {
		return nil, fmt.Errorf("failed to find ESP partition:\n%w", err)
	}

	bootPartition, err := findBootPartitionFromEsp(espPartition, partitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find boot partition:\n%w", err)
	}

	cmdline, err := extractKernelCmdlineFromGrub(bootPartition, buildDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to extract kernel arguments from grub.cfg:\n%w", err)
	} else if !os.IsNotExist(err) {
		return cmdline, nil
	}

	cmdline, err = extractKernelCmdlineFromUki(espPartition, buildDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to extract kernel arguments from UKI:\n%w", err)
	} else if os.IsNotExist(err) {
		return nil, fmt.Errorf("no kernel arguments found from either grub.cfg or UKI")
	}

	return cmdline, nil
}

func extractKernelCmdlineFromUki(espPartition *diskutils.PartitionInfo,
	buildDir string,
) ([]grubConfigLinuxArg, error) {
	tmpDirEsp := filepath.Join(buildDir, tmpEspPartitionDirName)
	espPartitionMount, err := safemount.NewMount(espPartition.Path, tmpDirEsp, espPartition.FileSystemType, 0, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount ESP partition (%s):\n%w", espPartition.Path, err)
	}
	defer espPartitionMount.Close()

	kernelToArgs, err := extractKernelCmdlineFromUkiEfis(tmpDirEsp, buildDir)
	if err != nil {
		return nil, err
	}

	// Assumes only one UKI is needed, uses the first entry in the map.
	var firstCmdline string
	for _, cmdline := range kernelToArgs {
		firstCmdline = cmdline
		break
	}

	tokens, err := grub.TokenizeConfig(firstCmdline)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize kernel command-line from UKI:\n%w", err)
	}

	args, err := ParseCommandLineArgs(tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel command-line from UKI:\n%w", err)
	}

	err = espPartitionMount.CleanClose()
	if err != nil {
		return nil, fmt.Errorf("failed to close espPartitionMount:\n%w", err)
	}

	return args, nil
}

func getUkiFiles(espPath string) ([]string, error) {
	espLinuxPath := filepath.Join(espPath, UkiOutputDir)
	ukiFiles, err := filepath.Glob(filepath.Join(espLinuxPath, "vmlinuz-*.efi"))
	if err != nil {
		return nil, fmt.Errorf("failed to search for UKI images in ESP partition:\n%w", err)
	}

	return ukiFiles, nil
}

func extractKernelCmdlineFromUkiEfis(espPath string, buildDir string) (map[string]string, error) {
	ukiFiles, err := getUkiFiles(espPath)
	if err != nil {
		return nil, err
	}

	kernelToArgsString := make(map[string]string)
	for _, ukiFile := range ukiFiles {
		kernelName, err := getKernelNameFromUki(ukiFile)
		if err != nil {
			return nil, fmt.Errorf("failed to extract kernel name from UKI file (%s):\n%w", ukiFile, err)
		}

		cmdlineContent, err := extractCmdlineFromUkiWithObjcopy(ukiFile, buildDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract cmdline from UKI file (%s):\n%w", ukiFile, err)
		}

		kernelToArgsString[kernelName] = string(cmdlineContent)
	}

	return kernelToArgsString, nil
}

func extractCmdlineFromUkiWithObjcopy(originalPath, buildDir string) (string, error) {
	// Create a temporary copy of UKI files to avoid modifying the original file,
	// since objcopy might tamper with signatures or hashes.
	tempCopy, err := os.CreateTemp(buildDir, "uki-copy-*.efi")
	if err != nil {
		return "", fmt.Errorf("failed to create temp UKI copy:\n%w", err)
	}
	defer os.Remove(tempCopy.Name())

	input, err := os.ReadFile(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to read UKI file:\n%w", err)
	}
	if err := os.WriteFile(tempCopy.Name(), input, 0o644); err != nil {
		return "", fmt.Errorf("failed to write temp UKI file:\n%w", err)
	}

	cmdlinePath, err := os.CreateTemp(buildDir, "cmdline-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp cmdline file:\n%w", err)
	}
	cmdlinePath.Close()
	defer os.Remove(cmdlinePath.Name())

	_, _, err = shell.Execute("objcopy", "--dump-section", ".cmdline="+cmdlinePath.Name(), tempCopy.Name())
	if err != nil {
		return "", fmt.Errorf("objcopy failed:\n%w", err)
	}

	content, err := os.ReadFile(cmdlinePath.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read kernel cmdline args from dumped file (%s):\n%w", cmdlinePath.Name(), err)
	}

	return string(content), nil
}

func extractKernelCmdlineFromGrub(bootPartition *diskutils.PartitionInfo,
	buildDir string,
) ([]grubConfigLinuxArg, error) {
	tmpDirBoot := filepath.Join(buildDir, tmpBootPartitionDirName)
	bootPartitionMount, err := safemount.NewMount(bootPartition.Path, tmpDirBoot, bootPartition.FileSystemType, unix.MS_RDONLY, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount boot partition (%s):\n%w", bootPartition.Path, err)
	}
	defer bootPartitionMount.Close()

	grubCfgPath := filepath.Join(tmpDirBoot, DefaultGrubCfgPath)
	kernelToArgs, err := extractKernelCmdlineFromGrubFile(grubCfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read grub.cfg:\n%w", err)
	}

	err = bootPartitionMount.CleanClose()
	if err != nil {
		return nil, fmt.Errorf("failed to close bootPartitionMount:\n%w", err)
	}

	for _, args := range kernelToArgs {
		// Pick the first set of the args.
		// (Hopefully they are all the same.)
		return args, nil
	}

	return nil, fmt.Errorf("no kernel args found in grub.cfg file")
}

// Extracts the kernel args for each kernel from the grub.cfg file.
// Returns a mapping from kernel version to list of kernel args.
func extractKernelCmdlineFromGrubFile(grubCfgPath string) (map[string][]grubConfigLinuxArg, error) {
	grubCfgContent, err := file.Read(grubCfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read grub.cfg file at (%s):\n%w", grubCfgPath, err)
	}

	lines, err := FindNonRecoveryLinuxLine(grubCfgContent)
	if err != nil {
		return nil, fmt.Errorf("failed to find linux command lines in grub.cfg:\n%w", err)
	}

	kernelToArgs := make(map[string][]grubConfigLinuxArg)
	for _, line := range lines {
		if len(line.Tokens) < 3 {
			return nil, fmt.Errorf("linux line in grub.cfg file has less than 3 args")
		}

		kernel := line.Tokens[1].RawContent
		kernel = strings.TrimPrefix(kernel, "/")

		argTokens := line.Tokens[2:]
		args, err := ParseCommandLineArgs(argTokens)
		if err != nil {
			return nil, fmt.Errorf("failed to parse linux command lines args (%s):\n%w", kernel, err)
		}

		kernelToArgs[kernel] = args
	}

	return kernelToArgs, nil
}

func extractVerityPartitionId(cmdline []grubConfigLinuxArg, verityDataArg string,
) (imagecustomizerapi.MountIdentifierType, string, error) {
	identifier, err := findKernelCommandLineArgValue(cmdline, verityDataArg)
	if err != nil {
		return "", "", fmt.Errorf("failed to find or parse (%s) argument:\n%w", verityDataArg, err)
	}

	if identifier == "" {
		return "", "", fmt.Errorf("verity identifier (%s) not found in kernel command-line", verityDataArg)
	}

	idType, value, err := parseSourcePartition(identifier)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse verity identifier (%s):\n%w", identifier, err)
	}

	return idType, value, nil
}

func parseSourcePartition(source string) (imagecustomizerapi.MountIdentifierType, string, error) {
	extendedType, id, err := parseExtendedSourcePartition(source)
	if err != nil {
		return imagecustomizerapi.MountIdentifierTypeDefault, "", err
	}

	// Map ExtendedMountIdentifierType to MountIdentifierType.
	var mountIdType imagecustomizerapi.MountIdentifierType
	switch extendedType {
	case ExtendedMountIdentifierTypeUuid:
		mountIdType = imagecustomizerapi.MountIdentifierTypeUuid
	case ExtendedMountIdentifierTypePartUuid:
		mountIdType = imagecustomizerapi.MountIdentifierTypePartUuid
	case ExtendedMountIdentifierTypePartLabel:
		mountIdType = imagecustomizerapi.MountIdentifierTypePartLabel
	default:
		return imagecustomizerapi.MountIdentifierTypeDefault, "", fmt.Errorf("unsupported identifier type: %v", extendedType)
	}

	return mountIdType, id, nil
}

func parseExtendedSourcePartition(source string) (ExtendedMountIdentifierType, string, error) {
	uuid, isUuid := strings.CutPrefix(source, "UUID=")
	if isUuid {
		return ExtendedMountIdentifierTypeUuid, uuid, nil
	}

	partUuid, isPartUuid := strings.CutPrefix(source, "PARTUUID=")
	if isPartUuid {
		return ExtendedMountIdentifierTypePartUuid, partUuid, nil
	}

	partLabel, isPartLabel := strings.CutPrefix(source, "PARTLABEL=")
	if isPartLabel {
		return ExtendedMountIdentifierTypePartLabel, partLabel, nil
	}

	if strings.HasPrefix(source, "/dev") {
		return ExtendedMountIdentifierTypeDev, source, nil
	}

	err := fmt.Errorf("unknown fstab source type (%s)", source)
	return ExtendedMountIdentifierTypeDefault, "", err
}

func getImageBootType(imageConnection *imageconnection.ImageConnection) (imagecustomizerapi.BootType, error) {
	diskPartitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return "", err
	}

	return getImageBootTypeHelper(diskPartitions)
}

func getImageBootTypeHelper(diskPartitions []diskutils.PartitionInfo) (imagecustomizerapi.BootType, error) {
	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return "", err
	}

	switch systemBootPartition.PartitionTypeUuid {
	case diskutils.EfiSystemPartitionTypeUuid:
		return imagecustomizerapi.BootTypeEfi, nil

	case diskutils.BiosBootPartitionTypeUuid:
		return imagecustomizerapi.BootTypeLegacy, nil

	default:
		return "", fmt.Errorf("internal error: unexpected system boot partition UUID (%s)",
			systemBootPartition.PartitionTypeUuid)
	}
}

func getNonSpecialChrootMountPoints(imageChroot *safechroot.Chroot) []*safechroot.MountPoint {
	return sliceutils.FindMatches(imageChroot.GetMountPoints(),
		func(mountPoint *safechroot.MountPoint) bool {
			switch mountPoint.GetTarget() {
			case "/dev", "/proc", "/sys", "/run", "/dev/pts", "/tmp":
				// Skip special directories.
				return false

			default:
				return true
			}
		},
	)
}

// Extract the partition number from the partition path.
// Ideally, we would use `lsblk --output PARTN` instead of this. But that is only available in util-linux v2.39+.
func getPartitionNum(partitionLoopDevice string) (int, error) {
	match := partitionNumberRegex.FindStringSubmatch(partitionLoopDevice)
	if match == nil {
		return 0, fmt.Errorf("failed to find partition number in partition dev path (%s)", partitionLoopDevice)
	}

	numStr := match[1]

	num, err := strconv.Atoi(numStr)
	if match == nil {
		return 0, fmt.Errorf("failed to parse partition number (%s):\n%w", numStr, err)
	}

	return num, nil
}
