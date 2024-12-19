// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"golang.org/x/sys/unix"
)

var (
	bootPartitionRegex = regexp.MustCompile(`(?m)^search -n -u ([a-zA-Z0-9\-]+) -s$`)

	// Extract the partition number from the loopback partition path.
	partitionNumberRegex = regexp.MustCompile(`^/dev/loop\d+p(\d+)$`)
)

func findPartitions(buildDir string, diskDevice string) ([]*safechroot.MountPoint, error) {
	var err error

	diskPartitions, err := diskutils.GetDiskPartitions(diskDevice)
	if err != nil {
		return nil, err
	}

	rootfsPartition, err := findRootfsPartition(diskPartitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find rootfs partition:\n%w", err)
	}

	mountPoints, err := findMountsFromRootfs(rootfsPartition, diskPartitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read fstab entries from rootfs partition:\n%w", err)
	}

	return mountPoints, nil
}

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
	tmpDir := filepath.Join(buildDir, tmpParitionDirName+"-esp")

	// Mount the EFI System Partition.
	efiSystemPartitionMount, err := safemount.NewMount(efiSystemPartition.Path, tmpDir, efiSystemPartition.FileSystemType, 0, "", true)
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

	tmpDir := filepath.Join(buildDir, tmpParitionDirName)

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
		partitionMount, err := safemount.NewMount(diskPartition.Path, tmpDir, diskPartition.FileSystemType, 0,
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

func findMountsFromRootfs(rootfsPartition *diskutils.PartitionInfo, diskPartitions []diskutils.PartitionInfo,
	buildDir string,
) ([]*safechroot.MountPoint, error) {
	logger.Log.Debugf("Reading fstab entries")

	tmpDir := filepath.Join(buildDir, tmpParitionDirName)

	// Temporarily mount the rootfs partition so that the fstab file can be read.
	rootfsPartitionMount, err := safemount.NewMount(rootfsPartition.Path, tmpDir, rootfsPartition.FileSystemType, 0, "",
		true)
	if err != nil {
		return nil, fmt.Errorf("failed to mount rootfs partition (%s):\n%w", rootfsPartition.Path, err)
	}
	defer rootfsPartitionMount.Close()

	// Read the fstab file.
	fstabPath := filepath.Join(tmpDir, "/etc/fstab")

	mountPoints, err := findMountsFromFstabFile(fstabPath, diskPartitions, buildDir)
	if err != nil {
		return nil, err
	}

	// Close the rootfs partition mount.
	err = rootfsPartitionMount.CleanClose()
	if err != nil {
		return nil, fmt.Errorf("failed to close rootfs partition mount (%s):\n%w", rootfsPartition.Path, err)
	}

	return mountPoints, nil
}

func findMountsFromFstabFile(fstabPath string, diskPartitions []diskutils.PartitionInfo, buildDir string,
) ([]*safechroot.MountPoint, error) {
	// Read the fstab file.
	fstabEntries, err := diskutils.ReadFstabFile(fstabPath)
	if err != nil {
		return nil, err
	}

	mountPoints, err := fstabEntriesToMountPoints(fstabEntries, diskPartitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find mount info for fstab file entries:\n%w", err)
	}

	return mountPoints, nil
}

func fstabEntriesToMountPoints(fstabEntries []diskutils.FstabEntry, diskPartitions []diskutils.PartitionInfo, buildDir string,
) ([]*safechroot.MountPoint, error) {
	filteredFstabEntries := filterOutSpecialPartitions(fstabEntries)

	// Convert fstab entries into mount points.
	var mountPoints []*safechroot.MountPoint
	var foundRoot bool
	for _, fstabEntry := range filteredFstabEntries {
		source, err := findSourcePartition(fstabEntry.Source, diskPartitions, buildDir)
		if err != nil {
			return nil, err
		}

		// Unset read-only flag so that read-only partitions can be customized.
		vfsOptions := fstabEntry.VfsOptions & ^diskutils.MountFlags(unix.MS_RDONLY)

		var mountPoint *safechroot.MountPoint
		if fstabEntry.Target == "/" {
			mountPoint = safechroot.NewPreDefaultsMountPoint(
				source, fstabEntry.Target, fstabEntry.FsType,
				uintptr(vfsOptions), fstabEntry.FsOptions)

			foundRoot = true
		} else {
			mountPoint = safechroot.NewMountPoint(
				source, fstabEntry.Target, fstabEntry.FsType,
				uintptr(vfsOptions), fstabEntry.FsOptions)
		}

		mountPoints = append(mountPoints, mountPoint)
	}

	if !foundRoot {
		return nil, fmt.Errorf("image has invalid fstab file: no root partition found")
	}

	return mountPoints, nil
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

func findSourcePartition(source string, partitions []diskutils.PartitionInfo, buildDir string) (string, error) {
	_, partition, _, err := findSourcePartitionHelper(source, partitions, buildDir)
	if err != nil {
		return "", err
	}

	return partition.Path, nil
}

func findSourcePartitionHelper(source string,
	partitions []diskutils.PartitionInfo, buildDir string,
) (imagecustomizerapi.MountIdentifierType, diskutils.PartitionInfo, int, error) {
	mountIdType, mountId, err := parseSourcePartition(source)
	if err != nil {
		return imagecustomizerapi.MountIdentifierTypeDefault, diskutils.PartitionInfo{}, 0, err
	}

	partition, partitionIndex, err := findPartition(mountIdType, mountId, partitions, buildDir)
	if err != nil {
		return imagecustomizerapi.MountIdentifierTypeDefault, diskutils.PartitionInfo{}, 0, err
	}

	return mountIdType, partition, partitionIndex, nil
}

func findPartition(mountIdType imagecustomizerapi.MountIdentifierType, mountId string,
	partitions []diskutils.PartitionInfo, buildDir string,
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
		case imagecustomizerapi.MountIdentifierTypeDevMapper:
			espPartition, err := findSystemBootPartition(partitions)
			if err != nil {
				return diskutils.PartitionInfo{}, 0, err
			}

			bootPartition, err := findBootPartitionFromEsp(espPartition, partitions, buildDir)
			if err != nil {
				return diskutils.PartitionInfo{}, 0, err
			}

			// Temporarily mount the boot partition so that the grub config file can be read.
			tmpDirBoot := filepath.Join(buildDir, "tmp-boot-partition")
			bootPartitionMount, err := safemount.NewMount(bootPartition.Path, tmpDirBoot, bootPartition.FileSystemType, 0, "", true)
			if err != nil {
				return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to mount boot partition (%s):\n%w", bootPartition.Path, err)
			}
			defer bootPartitionMount.Close()

			// Mount ESP partition to check for UKI.
			tmpDirEsp := filepath.Join(buildDir, "tmp-esp-partition")
			espPartitionMount, err := safemount.NewMount(espPartition.Path, tmpDirEsp, espPartition.FileSystemType, 0, "", true)
			if err != nil {
				return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to mount ESP partition (%s):\n%w", espPartition.Path, err)
			}
			defer espPartitionMount.Close()

			// Check for UKI image under /EFI/Linux.
			espLinuxPath := filepath.Join(tmpDirEsp, "EFI/Linux")
			ukiFiles, err := filepath.Glob(filepath.Join(espLinuxPath, "vmlinuz-*.efi"))
			if err != nil {
				return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to search for UKI images in ESP partition: %w", err)
			}

			if len(ukiFiles) > 0 {
				logger.Log.Infof("Found UKI image(s): %v", ukiFiles)

				// Dump kernel cmdline args from UKI.
				cmdlinePath := filepath.Join(buildDir, "cmdline.txt")
				cmd := exec.Command("objcopy", "--dump-section", ".cmdline="+cmdlinePath, ukiFiles[0])
				logger.Log.Debugf("Executing command: %s", strings.Join(cmd.Args, " "))
				if err := cmd.Run(); err != nil {
					logger.Log.Errorf("Command failed: %s", err)
					return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to dump kernel cmdline args from UKI: %w", err)
				}

				cmdlineContent, err := os.ReadFile(cmdlinePath)
				if err != nil {
					return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to read kernel cmdline args from dumped file: %w", err)
				}

				// Extract PARTUUID from kernel cmdline args.
				argsParts := strings.Split(string(cmdlineContent), " ")
				for _, part := range argsParts {
					if strings.HasPrefix(part, "systemd.verity_root_data=PARTUUID=") {
						extractedMountId := strings.TrimPrefix(part, "systemd.verity_root_data=PARTUUID=")
						matches = partition.PartUuid == extractedMountId
						break
					}
				}
			} else {
				grubCfgPath := filepath.Join(tmpDirBoot, "/grub2/grub.cfg")
				kernelToArgs, err := extractKernelToArgsFromGrub(grubCfgPath)
				if err != nil {
					return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to extract kernel arguments from grub.cfg: %w", err)
				}

				for _, args := range kernelToArgs {
					argsParts := strings.Split(args, " ")
					for _, part := range argsParts {
						if strings.HasPrefix(part, "systemd.verity_root_data=PARTUUID=") {
							extractedMountId := strings.TrimPrefix(part, "systemd.verity_root_data=PARTUUID=")
							matches = partition.PartUuid == extractedMountId
							break
						}
					}
				}
			}

			err = espPartitionMount.CleanClose()
			if err != nil {
				return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to close bootPartitionMount: %w", err)
			}

			err = bootPartitionMount.CleanClose()
			if err != nil {
				return diskutils.PartitionInfo{}, 0, fmt.Errorf("failed to close bootPartitionMount: %w", err)
			}
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

func parseSourcePartition(source string) (imagecustomizerapi.MountIdentifierType, string, error) {
	uuid, isUuid := strings.CutPrefix(source, "UUID=")
	if isUuid {
		return imagecustomizerapi.MountIdentifierTypeUuid, uuid, nil
	}

	partUuid, isPartUuid := strings.CutPrefix(source, "PARTUUID=")
	if isPartUuid {
		return imagecustomizerapi.MountIdentifierTypePartUuid, partUuid, nil
	}

	partLabel, isPartLabel := strings.CutPrefix(source, "PARTLABEL=")
	if isPartLabel {
		return imagecustomizerapi.MountIdentifierTypePartLabel, partLabel, nil
	}

	if strings.HasPrefix(source, "/dev/mapper") {
		return imagecustomizerapi.MountIdentifierTypeDevMapper, source, nil
	}

	err := fmt.Errorf("unknown fstab source type (%s)", source)
	return imagecustomizerapi.MountIdentifierTypeDefault, "", err
}

func findRootMountIdTypeFromFstabFile(imageConnection *ImageConnection,
) (imagecustomizerapi.MountIdentifierType, error) {
	fstabPath := filepath.Join(imageConnection.chroot.RootDir(), "etc/fstab")

	// Read the fstab file.
	fstabEntries, err := diskutils.ReadFstabFile(fstabPath)
	if err != nil {
		return imagecustomizerapi.MountIdentifierTypeDefault, err
	}

	rootMountMatches := sliceutils.FindMatches(fstabEntries, func(fstabEntry diskutils.FstabEntry) bool {
		return fstabEntry.Target == "/"
	})
	if len(rootMountMatches) < 1 {
		err := fmt.Errorf("failed to find root mount (/) in fstab file")
		return imagecustomizerapi.MountIdentifierTypeDefault, err
	}
	if len(rootMountMatches) > 1 {
		err := fmt.Errorf("too many root mounts (/) in fstab file")
		return imagecustomizerapi.MountIdentifierTypeDefault, err
	}

	rootMount := rootMountMatches[0]

	rootMountIdType, _, err := parseSourcePartition(rootMount.Source)
	if err != nil {
		err := fmt.Errorf("failed to get mount ID type of root (/) from fstab file:\n%w", err)
		return imagecustomizerapi.MountIdentifierTypeDefault, err
	}

	return rootMountIdType, nil
}

func getImageBootType(imageConnection *ImageConnection) (imagecustomizerapi.BootType, error) {
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
			case "/dev", "/proc", "/sys", "/run", "/dev/pts":
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

func refreshPartitions(diskDevPath string) error {
	err := shell.ExecuteLiveWithErr(1 /*stderrLines*/, "flock", "--timeout", "5", diskDevPath,
		"partprobe", "-s", diskDevPath)
	if err != nil {
		return fmt.Errorf("partprobe failed:\n%w", err)
	}

	return nil
}
