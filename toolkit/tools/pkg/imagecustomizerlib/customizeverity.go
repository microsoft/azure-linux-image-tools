// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/ptrutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
)

const (
	veritySignedRootHashFilesDir = "/boot"
)

func enableVerityPartition(verity []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot,
) (bool, error) {
	var err error

	if len(verity) <= 0 {
		return false, nil
	}

	logger.Log.Infof("Enable verity")

	err = validateVerityDependencies(imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to validate package dependencies for verity:\n%w", err)
	}

	// Integrate systemd veritysetup dracut module into initramfs img.
	systemdVerityDracutModule := "systemd-veritysetup"
	dmVerityDracutDriver := "dm-verity"
	err = addDracutModuleAndDriver(systemdVerityDracutModule, dmVerityDracutDriver, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to add dracut modules for verity:\n%w", err)
	}

	err = updateFstabForVerity(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to update fstab file for verity:\n%w", err)
	}

	err = prepareGrubConfigForVerity(imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to prepare grub config files for verity:\n%w", err)
	}

	return true, nil
}

func updateFstabForVerity(verityList []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot) error {
	var err error

	fstabFile := filepath.Join(imageChroot.RootDir(), "etc", "fstab")
	fstabEntries, err := diskutils.ReadFstabFile(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to read fstab file: %v", err)
	}

	// Update fstab entries so that verity mounts point to verity device paths.
	for _, verity := range verityList {
		if verity.FileSystem == nil || verity.FileSystem.MountPoint == nil {
			// No mount point assigned to verity device.
			continue
		}

		mountPath := verity.FileSystem.MountPoint.Path

		for j := range fstabEntries {
			entry := &fstabEntries[j]
			if entry.Target == mountPath {
				// Replace mount's source with verity device.
				entry.Source = verityDevicePath(verity)
			}
		}
	}

	// Write the updated fstab entries back to the fstab file
	err = diskutils.WriteFstabFile(fstabEntries, fstabFile)
	if err != nil {
		return err
	}

	return nil
}

func prepareGrubConfigForVerity(imageChroot *safechroot.Chroot) error {
	bootCustomizer, err := NewBootCustomizer(imageChroot)
	if err != nil {
		return err
	}

	err = bootCustomizer.PrepareForVerity()
	if err != nil {
		return err
	}

	err = bootCustomizer.WriteToFile(imageChroot)
	if err != nil {
		return err
	}

	return nil
}

func updateGrubConfigForVerity(rootfsVerity imagecustomizerapi.Verity, rootHash string, grubCfgFullPath string,
	partIdToPartUuid map[string]string, partitions []diskutils.PartitionInfo,
	provideRootHashSignatureArgument string, requireRootHashSignatureArgument string, bootPartitionUuid string,
) error {
	var err error

	// Format the dataPartitionId and hashPartitionId using the helper function.
	formattedDataPartition, err := systemdFormatPartitionId(rootfsVerity.DataDeviceId,
		rootfsVerity.DataDeviceMountIdType, partIdToPartUuid, partitions)
	if err != nil {
		return err
	}
	formattedHashPartition, err := systemdFormatPartitionId(rootfsVerity.HashDeviceId,
		rootfsVerity.HashDeviceMountIdType, partIdToPartUuid, partitions)
	if err != nil {
		return err
	}

	formattedCorruptionOption, err := systemdFormatCorruptionOption(rootfsVerity.CorruptionOption)
	if err != nil {
		return err
	}

	bootPartitionUuidArgument := ""
	// if provideRootHashSignatureArgument != "" || requireRootHashSignatureArgument != "" {
	bootPartitionUuidArgument = "pre.verity.mount=" + bootPartitionUuid
	// }

	newArgs := []string{
		"rd.systemd.verity=1",
		fmt.Sprintf("roothash=%s", rootHash),
		fmt.Sprintf("systemd.verity_root_data=%s", formattedDataPartition),
		fmt.Sprintf("systemd.verity_root_hash=%s", formattedHashPartition),
		fmt.Sprintf("systemd.verity_root_options=%s", formattedCorruptionOption),
		fmt.Sprintf("%s", provideRootHashSignatureArgument),
		fmt.Sprintf("%s", requireRootHashSignatureArgument),
		fmt.Sprintf("%s", bootPartitionUuidArgument),
	}

	grub2Config, err := file.Read(grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("failed to read grub config:\n%w", err)
	}

	// Note: If grub-mkconfig is being used, then we can't add the verity command-line args to /etc/default/grub and
	// call grub-mkconfig, since this would create a catch-22 with the verity root partition hash.
	// So, instead we just modify the /boot/grub2/grub.cfg file directly.
	grubMkconfigEnabled := isGrubMkconfigConfig(grub2Config)

	grub2Config, err = updateKernelCommandLineArgs(grub2Config, []string{"rd.systemd.verity", "roothash",
		"systemd.verity_root_data", "systemd.verity_root_hash", "systemd.verity_root_options"}, newArgs)
	if err != nil {
		return fmt.Errorf("failed to set verity kernel command line args:\n%w", err)
	}

	rootDevicePath := verityDevicePath(rootfsVerity)

	if grubMkconfigEnabled {
		grub2Config, err = updateKernelCommandLineArgs(grub2Config, []string{"root"},
			[]string{"root=" + rootDevicePath})
		if err != nil {
			return fmt.Errorf("failed to set verity root command-line arg:\n%w", err)
		}
	} else {
		grub2Config, err = replaceSetCommandValue(grub2Config, "rootdevice", rootDevicePath)
		if err != nil {
			return fmt.Errorf("failed to set verity root device:\n%w", err)
		}
	}

	err = file.Write(grub2Config, grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("failed to write updated grub config:\n%w", err)
	}

	return nil
}

func verityDevicePath(verity imagecustomizerapi.Verity) string {
	return verityDevicePathFromName(verity.Name)
}

func verityDevicePathFromName(name string) string {
	return imagecustomizerapi.DeviceMapperPath + "/" + name
}

// idToPartitionBlockDevicePath returns the block device path for a given idType and id.
func idToPartitionBlockDevicePath(configDeviceId string,
	diskPartitions []diskutils.PartitionInfo, partIdToPartUuid map[string]string,
) (string, error) {
	// Iterate over each partition to find the matching id.
	for _, partition := range diskPartitions {
		if partitionMatchesDeviceId(configDeviceId, partition, partIdToPartUuid) {
			return partition.Path, nil
		}
	}

	// If no partition is found with the given id.
	return "", fmt.Errorf("no partition found with id (%s)", configDeviceId)
}

func partitionMatchesDeviceId(configDeviceId string, partition diskutils.PartitionInfo,
	partIdToPartUuid map[string]string,
) bool {
	partUuid := partIdToPartUuid[configDeviceId]
	return partition.PartUuid == partUuid
}

// systemdFormatPartitionId formats the partition ID based on the ID type following systemd dm-verity style.
func systemdFormatPartitionId(configDeviceId string, mountIdType imagecustomizerapi.MountIdentifierType,
	partIdToPartUuid map[string]string, partitions []diskutils.PartitionInfo,
) (string, error) {
	partUuid := partIdToPartUuid[configDeviceId]

	partition, _, err := findPartition(imagecustomizerapi.MountIdentifierTypePartUuid, partUuid, partitions)
	if err != nil {
		return "", err
	}

	switch mountIdType {
	case imagecustomizerapi.MountIdentifierTypePartLabel:
		return fmt.Sprintf("%s=%s", "PARTLABEL", partition.PartLabel), nil

	case imagecustomizerapi.MountIdentifierTypeUuid:
		return fmt.Sprintf("%s=%s", "UUID", partition.Uuid), nil

	case imagecustomizerapi.MountIdentifierTypePartUuid, imagecustomizerapi.MountIdentifierTypeDefault:
		return fmt.Sprintf("%s=%s", "PARTUUID", partition.PartUuid), nil

	default:
		return "", fmt.Errorf("invalid idType provided (%s)", string(mountIdType))
	}
}

func systemdFormatCorruptionOption(corruptionOption imagecustomizerapi.CorruptionOption) (string, error) {
	switch corruptionOption {
	case imagecustomizerapi.CorruptionOptionDefault, imagecustomizerapi.CorruptionOptionIoError:
		return "", nil
	case imagecustomizerapi.CorruptionOptionIgnore:
		return "ignore-corruption", nil
	case imagecustomizerapi.CorruptionOptionPanic:
		return "panic-on-corruption", nil
	case imagecustomizerapi.CorruptionOptionRestart:
		return "restart-on-corruption", nil
	default:
		return "", fmt.Errorf("invalid corruptionOption provided (%s)", string(corruptionOption))
	}
}

func validateVerityDependencies(imageChroot *safechroot.Chroot) error {
	requiredRpms := []string{"lvm2"}

	// Iterate over each required package and check if it's installed.
	for _, pkg := range requiredRpms {
		logger.Log.Debugf("Checking if package (%s) is installed", pkg)
		if !isPackageInstalled(imageChroot, pkg) {
			return fmt.Errorf("package (%s) is not installed:\nthe following packages must be installed to use Verity: %v", pkg, requiredRpms)
		}
	}

	return nil
}

func generateSignedRootHashArtifacts(deviceId string, deviceRootHash string, outputVerityHashes bool, outputVerityHashesDir string,
	requireSignedRootfsRootHash bool, requireSignedRootHashes bool,
) (provideRootHashSignatureArgument string, requireRootHashSignatureArgument string, err error) {

	if !outputVerityHashes {
		return "", "", nil
	}

	rootHashFile := deviceId + ".hash"
	rootHashFileLocalPath := filepath.Join(outputVerityHashesDir, rootHashFile)
	rootHashSignedFileImagePath := filepath.Join("/boot", rootHashFile+".sig")

	err = os.MkdirAll(outputVerityHashesDir, os.ModePerm)
	if err != nil {
		return "", "", fmt.Errorf("failed to create root hashes directory (%s):\n%w", outputVerityHashesDir, err)
	}
	err = file.Write(deviceRootHash, rootHashFileLocalPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to write root hash to %s:\n%w", rootHashFileLocalPath, err)
	}

	if requireSignedRootfsRootHash {
		provideRootHashSignatureArgument = "systemd.verity_root_options=root_hash_signature=" + rootHashSignedFileImagePath
		if requireSignedRootHashes {
			requireRootHashSignatureArgument = "dm_verity.require_signatures=1"
		}
	}

	logger.Log.Debugf("---- debug ---- rootHashSignedFileImagePath=(%s)", rootHashSignedFileImagePath)
	logger.Log.Debugf("---- debug ---- provideRootHashSignatureArgument      =(%s)", provideRootHashSignatureArgument)
	logger.Log.Debugf("---- debug ---- requireRootHashSignatureArgument      =(%s)", requireRootHashSignatureArgument)

	return provideRootHashSignatureArgument, requireRootHashSignatureArgument, err
}

func generateSignedRootHashConfiguration(signedRootHashFiles []string) (imagecustomizerapi.AdditionalFileList, error) {
	additionalFiles := imagecustomizerapi.AdditionalFileList{}
	for _, localFile := range signedRootHashFiles {

		imageFile := filepath.Join(veritySignedRootHashFilesDir, filepath.Base(localFile))

		logger.Log.Debugf("---- debug ---- - src = %s", localFile)
		logger.Log.Debugf("---- debug ----   dst = %s", imageFile)

		additionalFile := imagecustomizerapi.AdditionalFile{
			Destination: imageFile,
			Source:      localFile,
			// ToDo: what permissions should we use?
			Permissions: ptrutils.PtrTo(imagecustomizerapi.FilePermissions(0o755)),
		}
		additionalFiles = append(additionalFiles, additionalFile)
	}
	return additionalFiles, nil
}
