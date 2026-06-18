// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/envfile"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/targetos"
	"go.opentelemetry.io/otel"
	"golang.org/x/sys/unix"
)

type installOSFunc func(imageChroot *safechroot.Chroot) error

func connectToExistingImage(ctx context.Context, imageFilePath string, buildDir string, chrootDirName string,
	includeDefaultMounts bool, readonly bool, readOnlyVerity bool, ignoreOverlays bool,
	distroHandler DistroHandler,
) (*imageconnection.ImageConnection, []fstabEntryPartNum, []verityDeviceMetadata, []string, DistroHandler, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "connect_to_existing_image")
	defer span.End()
	imageConnection := imageconnection.NewImageConnection()

	partitionsLayout, verityMetadata, readonlyPartUuids, distroHandler, err := connectToExistingImageHelper(imageConnection,
		imageFilePath, buildDir, chrootDirName, includeDefaultMounts, readonly, readOnlyVerity, ignoreOverlays,
		distroHandler)
	if err != nil {
		imageConnection.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to connect to OS image file:\n%w", err)
	}

	return imageConnection, partitionsLayout, verityMetadata, readonlyPartUuids, distroHandler, nil
}

func connectToExistingImageHelper(imageConnection *imageconnection.ImageConnection, imageFilePath string,
	buildDir string, chrootDirName string, includeDefaultMounts bool, readonly bool, readOnlyVerity bool,
	ignoreOverlays bool, distroHandler DistroHandler,
) ([]fstabEntryPartNum, []verityDeviceMetadata, []string, DistroHandler, error) {
	// Connect to image file using loopback device.
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	partitionTable, err := diskutils.ReadDiskPartitionTable(imageConnection.Loopback().DevicePath())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to read image's partition table:\n%w", err)
	}

	if partitionTable == nil {
		return nil, nil, nil, nil, fmt.Errorf("image does not contain a partition table")
	}

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return nil, nil, nil, nil, err
	}

	rootfsPartition, rootfsPath, err := findRootfsPartition(partitions, buildDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to find rootfs partition:\n%w", err)
	}

	fstabEntries, err := readFstabEntriesFromRootfs(rootfsPartition, buildDir, rootfsPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to read fstab entries from rootfs partition:\n%w", err)
	}

	partitionsLayout, verityMetadata, distroHandler, err := discoverPartitionLayout(fstabEntries, partitions, buildDir,
		ignoreOverlays, rootfsPartition, rootfsPath, distroHandler)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to discover partitions from fstab entries:\n%w", err)
	}

	if distroHandler == nil {
		// The Azure Container Linux image doesn't create empty directories for some of the standard Linux partitions,
		// like /dev. Hence, the `ConnectChroot` call below would fail when mounting as read-only. To workaround this
		// issue, we need to know which distro we are dealing with here, so that we can add a workaround for ACL.
		//
		// But for the ACL image, we could do the distro check after `ConnectChroot` and drastically simplify the code.
		// :-(
		targetOs, err := getInstalledTargetOsFromPartitionLayout(partitions, partitionsLayout, buildDir)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to determine the target OS:\n%w", err)
		}

		distroHandler, err = NewDistroHandler(targetOs)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	mountPoints, readonlyPartUuids, tempDirectories, err := partitionLayoutToMountPoints(partitionsLayout, partitions,
		readonly, readOnlyVerity, distroHandler, buildDir, includeDefaultMounts)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to find mount info for disk:\n%w", err)
	}

	imageConnection.AddOwnedDirectories(tempDirectories...)

	// Create chroot environment.
	imageChrootDir := filepath.Join(buildDir, chrootDirName)

	err = imageConnection.ConnectChroot(imageChrootDir, false, []string(nil), mountPoints, includeDefaultMounts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return partitionsLayout, verityMetadata, readonlyPartUuids, distroHandler, nil
}

func reconnectToExistingImage(ctx context.Context, imageFilePath string, buildDir string, chrootDirName string,
	includeDefaultMounts bool, readonly bool, readOnlyVerity bool, partitionsLayout []fstabEntryPartNum,
	distroHandler DistroHandler,
) (*imageconnection.ImageConnection, []string, error) {
	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "reconnect_to_existing_image")
	defer span.End()

	imageConnection, readonlyPartUuids, err := reconnectToExistingImageHelper(imageFilePath, buildDir,
		chrootDirName, includeDefaultMounts, readonly, readOnlyVerity, partitionsLayout, distroHandler)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to OS image file:\n%w", err)
	}
	return imageConnection, readonlyPartUuids, nil
}

func reconnectToExistingImageHelper(imageFilePath string, buildDir string, chrootDirName string,
	includeDefaultMounts bool, readonly bool, readOnlyVerity bool, partitionsLayout []fstabEntryPartNum,
	distroHandler DistroHandler,
) (*imageconnection.ImageConnection, []string, error) {
	ok := false

	imageConnection := imageconnection.NewImageConnection()
	defer func() {
		if !ok {
			imageConnection.Close()
		}
	}()

	// Connect to image file using loopback device.
	err := imageConnection.ConnectLoopback(imageFilePath)
	if err != nil {
		return nil, nil, err
	}

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return nil, nil, err
	}

	mountPoints, readonlyPartUuids, tempDirectories, err := partitionLayoutToMountPoints(partitionsLayout, partitions,
		readonly, readOnlyVerity, distroHandler, buildDir, includeDefaultMounts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find mount info for disk:\n%w", err)
	}

	imageConnection.AddOwnedDirectories(tempDirectories...)

	// Create chroot environment.
	imageChrootDir := filepath.Join(buildDir, chrootDirName)

	err = imageConnection.ConnectChroot(imageChrootDir, false, []string(nil), mountPoints, includeDefaultMounts)
	if err != nil {
		return nil, nil, err
	}

	ok = true
	return imageConnection, readonlyPartUuids, nil
}

func CreateNewImage(distroHandler DistroHandler, filename string, diskConfig imagecustomizerapi.Disk,
	fileSystems []imagecustomizerapi.FileSystem, buildDir string, chrootDirName string,
	installOS installOSFunc,
) (map[string]string, error) {
	imageConnection := imageconnection.NewImageConnection()
	defer imageConnection.Close()

	partIdToPartUuid, err := createNewImageHelper(distroHandler, imageConnection, filename, diskConfig, fileSystems,
		buildDir, chrootDirName, installOS)
	if err != nil {
		return nil, fmt.Errorf("failed to create new image:\n%w", err)
	}

	// Close image.
	err = imageConnection.CleanClose()
	if err != nil {
		return nil, err
	}

	return partIdToPartUuid, nil
}

func createNewImageHelper(distroHandler DistroHandler, imageConnection *imageconnection.ImageConnection,
	filename string, diskConfig imagecustomizerapi.Disk, fileSystems []imagecustomizerapi.FileSystem, buildDir string,
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
	partIdToPartUuid, tmpFstabFile, err := createImageBoilerplate(distroHandler, imageConnection, filename, buildDir,
		chrootDirName, imagerDiskConfig, imagerPartitionSettings, fileSystems)
	if err != nil {
		return nil, err
	}

	// Install the OS.
	err = installOS(imageConnection.Chroot())
	if err != nil {
		return nil, err
	}

	// Move the fstab file into the image.
	imageFstabFilePath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/fstab")

	err = file.Move(tmpFstabFile, imageFstabFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to move fstab into new image:\n%w", err)
	}

	return partIdToPartUuid, nil
}

func configureDiskBootLoader(imageConnection *imageconnection.ImageConnection,
	rootMountIdType imagecustomizerapi.MountIdentifierType, bootType imagecustomizerapi.BootType,
	selinuxConfig imagecustomizerapi.SELinux, kernelCommandLine imagecustomizerapi.KernelCommandLine,
	currentSELinuxMode imagecustomizerapi.SELinuxMode, forceGrubMkconfig bool, distroHandler DistroHandler,
	assetGrubDefFile string, grubEnvRelPath string, assetGrubStubFile string, grubStubDirs []string,
	allowHostGrubInstallFallback bool, grubInstallPackage string, grubModulesPackage string,
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

	useGrubMkconfig := forceGrubMkconfig
	if !forceGrubMkconfig {
		// Detect the boot configuration type to determine whether to use grub mkconfig.
		grubCfgContent, err := distroHandler.ReadGrub2ConfigFile(imageConnection.Chroot())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		bootConfigType, err := determineBootConfigType(grubCfgContent, imageConnection.Chroot(), distroHandler)
		if err != nil {
			return err
		}
		useGrubMkconfig = (bootConfigType == bootConfigTypeGrubMkconfig)
	}

	mountPointMap := make(map[string]string)
	for _, mountPoint := range imageConnection.Chroot().GetMountPoints() {
		mountPointMap[mountPoint.GetTarget()] = mountPoint.GetSource()
	}

	// Configure the boot loader.
	err = installutils.ConfigureDiskBootloaderWithRootMountIdType(imagerBootType, false, imagerRootMountIdType,
		imagerKernelCommandLine, imageConnection.Chroot(), imageConnection.Loopback().DevicePath(), mountPointMap,
		diskutils.EncryptedRootDevice{}, useGrubMkconfig, assetGrubDefFile, grubEnvRelPath, assetGrubStubFile,
		grubStubDirs, allowHostGrubInstallFallback, grubInstallPackage, grubModulesPackage)
	if err != nil {
		return fmt.Errorf("failed to install bootloader:\n%w", err)
	}

	return nil
}

func createImageBoilerplate(distroHandler DistroHandler, imageConnection *imageconnection.ImageConnection,
	filename string, buildDir string, chrootDirName string, imagerDiskConfig configuration.Disk,
	imagerPartitionSettings []configuration.PartitionSetting, fileSystems []imagecustomizerapi.FileSystem,
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
		distroHandler.GetTargetOs(), imageConnection.Loopback().DevicePath(), imagerDiskConfig,
		configuration.RootEncryption{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create partitions on disk (%s):\n%w", imageConnection.Loopback().DevicePath(), err)
	}

	// Create BTRFS subvolumes for any file systems that have them defined.
	err = createBtrfsSubvolumes(fileSystems, partIDToDevPathMap, partIDToFsTypeMap, buildDir)
	if err != nil {
		return nil, "", err
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

	// Create the temporary fstab file.
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

	partitionsLayout, _, _, err := discoverPartitionLayout(fstabEntries, diskPartitions, buildDir, false, nil, "",
		distroHandler)
	if err != nil {
		return nil, "", fmt.Errorf("failed to discover partitions from fstab entries:\n%w", err)
	}

	mountPoints, _, tempDirectories, err := partitionLayoutToMountPoints(partitionsLayout, diskPartitions, false, false,
		distroHandler, buildDir, false)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find mount info for disk:\n%w", err)
	}

	imageConnection.AddOwnedDirectories(tempDirectories...)

	// Create chroot environment.
	imageChrootDir := filepath.Join(buildDir, chrootDirName)

	err = imageConnection.ConnectChroot(imageChrootDir, false, nil, mountPoints, false)
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

func extractOSRelease(imageConnection *imageconnection.ImageConnection) (string, error) {
	// Try /etc/os-release first, then fall back to /usr/lib/os-release.
	// The fallback is per the os-release(5) spec and is needed for distros like
	// ACL where /etc is an overlay that may not be mounted during customization.
	osReleasePath := filepath.Join(imageConnection.Chroot().RootDir(), "etc/os-release")
	data, err := file.Read(osReleasePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("failed to read /etc/os-release:\n%w", err)
		}
		osReleasePath = filepath.Join(imageConnection.Chroot().RootDir(), "usr/lib/os-release")
		data, err = file.Read(osReleasePath)
		if err != nil {
			return "", fmt.Errorf("failed to read os-release (tried /etc/os-release and /usr/lib/os-release):\n%w", err)
		}
	}

	return string(data), nil
}

// clearBtrfsReadOnlyProperties clears the btrfs read-only subvolume property on
// any btrfs mount that IC mounted as read-write. Some distros (e.g. ACL) set
// this property at build time to make partitions immutable at runtime.
func clearBtrfsReadOnlyProperties(imageConnection *imageconnection.ImageConnection) error {
	for _, mp := range imageConnection.Chroot().GetMountPoints() {
		if mp.GetFSType() != "btrfs" {
			continue
		}

		// Skip mounts that IC intentionally mounted read-only.
		if (mp.GetFlags() & unix.MS_RDONLY) != 0 {
			continue
		}

		mountPath := filepath.Join(imageConnection.Chroot().RootDir(), mp.GetTarget())

		// Check if the subvolume is read-only.
		stdout, stderr, err := shell.Execute("btrfs", "property", "get", "-ts", mountPath, "ro")
		if err != nil {
			// Not all btrfs mounts have subvolumes; skip on error.
			logger.Log.Debugf("Skipping btrfs property check on %s: %v: %s", mp.GetTarget(), err, stderr)
			continue
		}

		if strings.TrimSpace(stdout) != "ro=true" {
			continue
		}

		logger.Log.Debugf("Clearing btrfs read-only property on %s", mp.GetTarget())
		err = shell.ExecuteLive(true, "btrfs", "property", "set", "-ts", mountPath, "ro", "false")
		if err != nil {
			return fmt.Errorf("failed to clear btrfs read-only property on %s:\n%w", mp.GetTarget(), err)
		}
	}

	return nil
}

// Get the target OS from the partition layout.
// Note: This function primarily exists as part of a workaround for the ACL image.
// Note: This function is similar to `detectDistroFromRootfs` but `detectDistroFromRootfs` has to run pre verity
// resolution. Whereas this function is post verity resolution. So it can also scan verity partitions, making it more
// robust.
func getInstalledTargetOsFromPartitionLayout(diskPartitions []diskutils.PartitionInfo,
	partitionsLayout []fstabEntryPartNum, buildDir string,
) (targetos.TargetOs, error) {
	for _, candidate := range targetos.OsReleaseFileCandidates {
		entryIndex, relativePath, found := getMostSpecificPath(candidate, slices.All(partitionsLayout),
			func(entry fstabEntryPartNum) string { return entry.FstabEntry.Target })
		if !found {
			return targetos.TargetOs{}, fmt.Errorf("failed to find fstab entry for path (path='%s')", candidate)
		}

		entry := partitionsLayout[entryIndex]
		fstabEntry := entry.FstabEntry

		diskPartition, foundDiskPartition := sliceutils.FindValueFunc(diskPartitions,
			func(diskPartition diskutils.PartitionInfo) bool {
				return diskPartition.PartUuid == entry.PartUuid
			})
		if !foundDiskPartition {
			err := fmt.Errorf("failed to find partition for fstab entry (partuuid='%s')", entry.PartUuid)
			return targetos.TargetOs{}, err
		}

		mountDir := filepath.Join(buildDir, tmpPartitionDirName)

		mount, err := safemount.NewMount(diskPartition.Path, mountDir, diskPartition.FileSystemType,
			uintptr(fstabEntry.VfsOptions), fstabEntry.FsOptions, true /*makeAndDeleteDir*/)
		if err != nil {
			return targetos.TargetOs{}, err
		}
		defer mount.Close()

		osReleasePath := filepath.Join(mountDir, relativePath)
		osReleaseFileExists, err := file.PathExists(osReleasePath)
		if err != nil {
			return targetos.TargetOs{}, fmt.Errorf("failed to check if os-release file exists (path='%s'):\n%w",
				osReleasePath, err)
		}

		if !osReleaseFileExists {
			err = mount.CleanClose()
			if err != nil {
				return targetos.TargetOs{}, err
			}

			continue
		}

		fields, err := envfile.ParseEnvFile(osReleasePath)
		if err != nil {
			return targetos.TargetOs{}, fmt.Errorf("failed to read os-release file (path='%s'):\n%w", osReleasePath,
				err)
		}

		err = mount.CleanClose()
		if err != nil {
			return targetos.TargetOs{}, err
		}

		targetOs, err := targetos.GetInstalledTargetOsFromEnvFields(fields, "os-release")
		if err != nil {
			return targetos.TargetOs{}, err
		}

		return targetOs, nil
	}

	return targetos.TargetOs{}, fmt.Errorf("failed to determine OS distro:\ncould not find os-release file")
}
