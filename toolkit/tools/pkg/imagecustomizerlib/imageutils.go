// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/targetos"
)

type installOSFunc func(imageChroot *safechroot.Chroot) error

func connectToExistingImage(imageFilePath string, buildDir string, chrootDirName string, includeDefaultMounts bool, tarFile string,
) (*ImageConnection, map[string]diskutils.FstabEntry, error) {
	imageConnection := NewImageConnection()

	partUuidToMountPath, err := connectToExistingImageHelper(imageConnection, imageFilePath, buildDir, chrootDirName, includeDefaultMounts, tarFile)
	if err != nil {
		imageConnection.Close()
		return nil, nil, err
	}
	return imageConnection, partUuidToMountPath, nil
}

func connectToExistingImageHelper(imageConnection *ImageConnection, imageFilePath string,
	buildDir string, chrootDirName string, includeDefaultMounts bool, tarFile string,
) (map[string]diskutils.FstabEntry, error) {
	// Connect to image file using loopback device.
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		return nil, err
	}

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return nil, err
	}

	fmt.Printf("**********Partitions: %v\n", partitions)

	rootfsPartition, err := findRootfsPartition(partitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find rootfs partition:\n%w", err)
	}

	fstabEntries, err := readFstabEntriesFromRootfs(rootfsPartition, partitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read fstab entries from rootfs partition:\n%w", err)
	}

	mountPoints, partUuidToFstabEntry, err := fstabEntriesToMountPoints(fstabEntries, partitions, buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find mount info for fstab file entries:\n%w", err)
	}
	// print mount points
	for _, mountPoint := range mountPoints {
		fmt.Printf("Mount point: %s, Source: %s\n", mountPoint.GetTarget(), mountPoint.GetSource())
	}

	// Create chroot environment.
	imageChrootDir := filepath.Join(buildDir, chrootDirName)

	err = imageConnection.ConnectChroot(imageChrootDir, false, nil, mountPoints, includeDefaultMounts, "")
	if err != nil {
		return nil, err
	}

	return partUuidToFstabEntry, nil
}

func CreateNewImage(targetOs targetos.TargetOs, filename string, diskConfig imagecustomizerapi.Disk,
	fileSystems []imagecustomizerapi.FileSystem, buildDir string, chrootDirName string,
	installOS installOSFunc,
) (map[string]string, string, error) {
	imageConnection := NewImageConnection()
	defer imageConnection.Close()

	partIdToPartUuid, err := createNewImageHelper(targetOs, imageConnection, filename, diskConfig, fileSystems,
		buildDir, chrootDirName, installOS)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new image:\n%w", err)
	}

	// // Close image.
	// err = imageConnection.CleanClose()
	// if err != nil {
	// 	return nil, err
	// }

	return partIdToPartUuid, imageConnection.loopback.DevicePath(), nil
}

func createNewImageHelper(targetOs targetos.TargetOs, imageConnection *ImageConnection, filename string,
	diskConfig imagecustomizerapi.Disk, fileSystems []imagecustomizerapi.FileSystem, buildDir string,
	chrootDirName string, installOS installOSFunc,
) (map[string]string, error) {
	// Convert config to image config types, so that the imager's utils can be used.
	imagerDiskConfig, err := diskConfigToImager(diskConfig, fileSystems)
	if err != nil {
		return nil, err
	}

	imagerPartitionSettings, err := partitionSettingsToImager(fileSystems)
	if err != nil {
		return nil, err
	}

	// Create imager boilerplate.
	partIdToPartUuid, tmpFstabFile, err := createImageBoilerplate(targetOs, imageConnection, filename, buildDir,
		chrootDirName, imagerDiskConfig, imagerPartitionSettings)
	if err != nil {
		return nil, err
	}

	// Install the OS.
	err = installOS(imageConnection.Chroot())
	if err != nil {
		return nil, err
	}

	// Move the fstab file into the image.
	imageFstabFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "/etc/fstab")

	err = file.Move(tmpFstabFile, imageFstabFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to move fstab into new image:\n%w", err)
	}

	return partIdToPartUuid, nil
}

func configureDiskBootLoader(imageConnection *ImageConnection, rootMountIdType imagecustomizerapi.MountIdentifierType,
	bootType imagecustomizerapi.BootType, selinuxConfig imagecustomizerapi.SELinux,
	kernelCommandLine imagecustomizerapi.KernelCommandLine, currentSELinuxMode imagecustomizerapi.SELinuxMode,
) error {
	imagerBootType, err := bootTypeToImager(bootType)
	if err != nil {
		return err
	}

	imagerKernelCommandLine, err := kernelCommandLineToImager(kernelCommandLine, selinuxConfig, currentSELinuxMode)
	if err != nil {
		return err
	}

	imagerRootMountIdType, err := mountIdentifierTypeToImager(rootMountIdType)
	if err != nil {
		return err
	}
	/*
		grubMkconfigEnabled, err := isGrubMkconfigEnabled(imageConnection.Chroot())
		if err != nil {
			return err
		}
	*/

	mountPointMap := make(map[string]string)
	for _, mountPoint := range imageConnection.Chroot().GetMountPoints() {
		mountPointMap[mountPoint.GetTarget()] = mountPoint.GetSource()
	}

	// Append /:/dev/loop12p2 /boot/efi:/dev/loop12p1 to the mount point map.
	// This is a workaround for the issue where the mount point is not set correctly in the image.
	diskdevPath := imageConnection.Loopback().DevicePath()

	mountPointMap["/boot/efi"] = diskdevPath + "p1"
	mountPointMap["/"] = diskdevPath + "p2"

	fmt.Println("Mount point map: new %v\n", mountPointMap)
	grubMkconfigEnabled := true
	// Configure the boot loader.
	err = installutils.ConfigureDiskBootloaderWithRootMountIdType(imagerBootType, false, imagerRootMountIdType,
		imagerKernelCommandLine, imageConnection.Chroot(), imageConnection.Loopback().DevicePath(),
		mountPointMap, diskutils.EncryptedRootDevice{}, grubMkconfigEnabled,
		!grubMkconfigEnabled)
	if err != nil {
		return fmt.Errorf("failed to install bootloader:\n%w", err)
	}

	return nil
}

func createImageBoilerplate(targetOs targetos.TargetOs, imageConnection *ImageConnection, filename string,
	buildDir string, chrootDirName string, imagerDiskConfig configuration.Disk,
	imagerPartitionSettings []configuration.PartitionSetting,
) (map[string]string, string, error) {
	// Create raw disk image file.
	err := diskutils.CreateSparseDisk(filename, imagerDiskConfig.MaxSize, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create empty disk file (%s):\n%w", filename, err)
	}

	// Connect raw disk image file.
	err = imageConnection.ConnectLoopback(filename)
	if err != nil {
		return nil, "", err
	}

	// Set up partitions.
	partIDToDevPathMap, partIDToFsTypeMap, _, err := diskutils.CreatePartitions(
		targetOs, imageConnection.Loopback().DevicePath(), imagerDiskConfig, configuration.RootEncryption{},
		true /*diskKnownToBeEmpty*/)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create partitions on disk (%s):\n%w", imageConnection.Loopback().DevicePath(), err)
	}

	// Read the disk partitions.
	diskPartitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return nil, "", err
	}

	// Create mapping from partition ID to partition UUID.
	partIdToPartUuid, err := createPartIdToPartUuidMap(partIDToDevPathMap, diskPartitions)
	if err != nil {
		return nil, "", err
	}

	// Create the fstab file.
	// This is done so that we can read back the file using findmnt, which conveniently splits the vfs and fs mount
	// options for us. If we wanted to handle this more directly, we could create a golang wrapper around libmount
	// (which is what findmnt uses). But we are already using the findmnt in other places.
	tmpFstabFile := filepath.Join(buildDir, chrootDirName+"_fstab")
	err = file.RemoveFileIfExists(tmpFstabFile)
	if err != nil {
		return nil, "", err
	}

	mountPointMap, mountPointToFsTypeMap, mountPointToMountArgsMap, _ := installutils.CreateMountPointPartitionMap(
		partIDToDevPathMap, partIDToFsTypeMap, imagerPartitionSettings,
	)

	mountList := sliceutils.MapToSlice(mountPointMap)

	// Sort the mounts so that they are mounted in the correct oder.
	sort.Slice(mountList, func(i, j int) bool {
		return mountList[i] < mountList[j]
	})

	err = installutils.UpdateFstabFile(tmpFstabFile, imagerPartitionSettings, mountList, mountPointMap,
		mountPointToFsTypeMap, mountPointToMountArgsMap, partIDToDevPathMap, partIDToFsTypeMap,
		false, /*hidepidEnabled*/
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write temp fstab file:\n%w", err)
	}

	// Read back the fstab file.
	fstabEntries, err := diskutils.ReadFstabFile(tmpFstabFile)
	if err != nil {
		return nil, "", err
	}

	mountPoints, _, err := fstabEntriesToMountPoints(fstabEntries, diskPartitions, buildDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find mount info for fstab file entries:\n%w", err)
	}

	// print mount points
	for _, mountPoint := range mountPoints {
		fmt.Printf("Mount point: %s, Source: %s\n", mountPoint.GetTarget(), mountPoint.GetSource())
	}
	// Create chroot environment.
	imageChrootDir := filepath.Join(buildDir, chrootDirName)
	err = imageConnection.ConnectChroot(imageChrootDir, false, nil, mountPoints, true, "")
	if err != nil {
		return nil, "", err
	}

	return partIdToPartUuid, tmpFstabFile, nil
}

func createPartIdToPartUuidMap(partIDToDevPathMap map[string]string, diskPartitions []diskutils.PartitionInfo,
) (map[string]string, error) {
	partIdToPartUuid := make(map[string]string)
	for partId, devPath := range partIDToDevPathMap {
		partition, found := sliceutils.FindValueFunc(diskPartitions, func(partition diskutils.PartitionInfo) bool {
			return devPath == partition.Path
		})
		if !found {
			return nil, fmt.Errorf("failed to find partition for device path (%s)", devPath)
		}

		partIdToPartUuid[partId] = partition.PartUuid
	}

	return partIdToPartUuid, nil
}

func extractOSRelease(imageConnection *ImageConnection) (string, error) {
	osReleasePath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/os-release")
	data, err := file.Read(osReleasePath)
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/os-release:\n%w", err)
	}

	return string(data), nil
}
