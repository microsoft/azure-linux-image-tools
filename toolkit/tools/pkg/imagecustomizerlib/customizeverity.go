// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/resources"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	systemdVerityDracutModule = "systemd-veritysetup"
	dmVerityDracutDriver      = "dm-verity"
	mountBootPartModule       = "mountbootpartition"

	// Dracut module directory path for verity boot partition support.
	VerityMountBootPartitionModuleDir = "/usr/lib/dracut/modules.d/90mountbootpartition"
	// Standard permission mode for dracut module directories.
	DracutModuleDirMode = 0755
	// Standard permission mode for executable scripts in dracut modules.
	DracutModuleScriptFileMode = 0755
)

func enableVerityPartition(ctx context.Context, verity []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot,
) (bool, error) {
	var err error

	if len(verity) <= 0 {
		return false, nil
	}

	logger.Log.Infof("Enable verity")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "enable_verity_partition")
	span.SetAttributes(
		attribute.Int("verity_count", len(verity)),
		attribute.String("verity_device_name", verity[0].Name),
	)
	defer span.End()

	err = validateVerityDependencies(imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to validate package dependencies for verity:\n%w", err)
	}

	// Integrate systemd veritysetup dracut module into initramfs img.
	err = addDracutModuleAndDriver(systemdVerityDracutModule, dmVerityDracutDriver, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to add dracut modules for verity:\n%w", err)
	}

	err = updateFstabForVerity(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to update fstab file for verity:\n%w", err)
	}

	err = prepareGrubConfigForVerity(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to prepare grub config files for verity:\n%w", err)
	}

	err = supportVerityHashSignature(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("failed to support hash signature for verity:\n%w", err)
	}

	return true, nil
}

func updateFstabForVerity(verityList []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot) error {
	fstabFile := filepath.Join(imageChroot.RootDir(), "etc", "fstab")
	fstabEntries, err := diskutils.ReadFstabFile(fstabFile)
	if err != nil {
		return fmt.Errorf("failed to read fstab file: %v", err)
	}

	// Update fstab entries so that verity mounts point to verity device paths.
	for _, verity := range verityList {
		mountPath := verity.MountPath

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

func prepareGrubConfigForVerity(verityList []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot) error {
	bootCustomizer, err := NewBootCustomizer(imageChroot)
	if err != nil {
		return err
	}

	for _, verity := range verityList {
		mountPath := verity.MountPath

		if mountPath == "/" {
			if err := bootCustomizer.PrepareForVerity(); err != nil {
				return err
			}
		}
	}

	if err := bootCustomizer.WriteToFile(imageChroot); err != nil {
		return err
	}

	return nil
}

func supportVerityHashSignature(verityList []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot) error {
	for _, verity := range verityList {
		if verity.HashSignaturePath == "" {
			continue
		}

		err := addDracutModule(mountBootPartModule, imageChroot)
		if err != nil {
			return fmt.Errorf("failed to add dracut modules for verity hash signature support:\n%w", err)
		}

		err = installVerityMountBootPartitionDracutModule(imageChroot.RootDir())
		if err != nil {
			return fmt.Errorf("failed to install verity dracut scripts:\n%w", err)
		}

		break
	}

	return nil
}

func installVerityMountBootPartitionDracutModule(installRoot string) error {
	targetDir := filepath.Join(installRoot, VerityMountBootPartitionModuleDir)

	filesToInstall := map[string]string{
		resources.VerityMountBootPartitionSetupFile:     filepath.Join(targetDir, "module-setup.sh"),
		resources.VerityMountBootPartitionGeneratorFile: filepath.Join(targetDir, "mountbootpartition-generator.sh"),
		resources.VerityMountBootPartitionGenRulesFile:  filepath.Join(targetDir, "mountbootpartition-genrules.sh"),
		resources.VerityMountBootPartitionScriptFile:    filepath.Join(targetDir, "mountbootpartition.sh"),
	}

	for src, dst := range filesToInstall {
		err := file.CopyResourceFile(resources.ResourcesFS, src, dst, DracutModuleDirMode, DracutModuleScriptFileMode)
		if err != nil {
			return fmt.Errorf("failed to install verity dracut file (%s): %w", dst, err)
		}
	}

	return nil
}

func updateGrubConfigForVerity(verityMetadata []verityDeviceMetadata, grubCfgFullPath string,
	partitions []diskutils.PartitionInfo, buildDir string, bootUuid string,
) error {
	var err error

	newArgs, err := constructVerityKernelCmdlineArgs(verityMetadata, partitions, buildDir, bootUuid)
	if err != nil {
		return fmt.Errorf("failed to generate verity kernel arguments:\n%w", err)
	}

	grub2Config, err := file.Read(grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("failed to read grub config:\n%w", err)
	}

	// Note: If grub-mkconfig is being used, then we can't add the verity command-line args to /etc/default/grub and
	// call grub-mkconfig, since this would create a catch-22 with the verity root partition hash.
	// So, instead we just modify the /boot/grub2/grub.cfg file directly.
	grubMkconfigEnabled := isGrubMkconfigConfig(grub2Config)

	grub2Config, err = updateKernelCommandLineArgsAll(grub2Config, []string{
		"rd.systemd.verity", "roothash", "systemd.verity_root_data",
		"systemd.verity_root_hash", "systemd.verity_root_options",
		"usrhash", "systemd.verity_usr_data", "systemd.verity_usr_hash",
		"systemd.verity_usr_options",
	}, newArgs)
	if err != nil {
		return fmt.Errorf("failed to set verity kernel command line args:\n%w", err)
	}

	rootExists := slices.ContainsFunc(verityMetadata, func(metadata verityDeviceMetadata) bool {
		return metadata.name == imagecustomizerapi.VerityRootDeviceName
	})
	if rootExists {
		rootDevicePath := imagecustomizerapi.VerityRootDevicePath

		if grubMkconfigEnabled {
			grub2Config, err = updateKernelCommandLineArgsAll(grub2Config, []string{"root"},
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
	}

	err = file.Write(grub2Config, grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("failed to write updated grub config:\n%w", err)
	}

	return nil
}

func constructVerityKernelCmdlineArgs(verityMetadata []verityDeviceMetadata,
	partitions []diskutils.PartitionInfo, buildDir string, bootUuid string,
) ([]string, error) {
	newArgs := []string{"rd.systemd.verity=1"}
	hasSignatureInjection := false

	for _, metadata := range verityMetadata {
		var hashArg, dataArg, optionsArg, hashKey string

		switch metadata.name {
		case imagecustomizerapi.VerityRootDeviceName:
			hashArg = "roothash"
			dataArg = "systemd.verity_root_data"
			hashKey = "systemd.verity_root_hash"
			optionsArg = "systemd.verity_root_options"

		case imagecustomizerapi.VerityUsrDeviceName:
			hashArg = "usrhash"
			dataArg = "systemd.verity_usr_data"
			hashKey = "systemd.verity_usr_hash"
			optionsArg = "systemd.verity_usr_options"

		default:
			return nil, fmt.Errorf("unsupported verity device (%s)", metadata.name)
		}

		formattedDataPartition, err := systemdFormatPartitionUuid(metadata.dataPartUuid,
			metadata.dataDeviceMountIdType, partitions, buildDir)
		if err != nil {
			return nil, err
		}

		formattedHashPartition, err := systemdFormatPartitionUuid(metadata.hashPartUuid,
			metadata.hashDeviceMountIdType, partitions, buildDir)
		if err != nil {
			return nil, err
		}

		formattedCorruptionOption, err := SystemdFormatCorruptionOption(metadata.corruptionOption)
		if err != nil {
			return nil, err
		}

		options := formattedCorruptionOption
		if metadata.hashSignaturePath != "" {
			options += fmt.Sprintf(",root-hash-signature=%s", metadata.hashSignaturePath)
			hasSignatureInjection = true
		}

		newArgs = append(newArgs,
			fmt.Sprintf("%s=%s", hashArg, metadata.rootHash),
			fmt.Sprintf("%s=%s", dataArg, formattedDataPartition),
			fmt.Sprintf("%s=%s", hashKey, formattedHashPartition),
			fmt.Sprintf("%s=%s", optionsArg, options),
		)
	}

	if hasSignatureInjection {
		newArgs = append(newArgs, fmt.Sprintf("pre.verity.mount=%s", bootUuid))
	}

	return newArgs, nil
}

func verityDevicePath(verity imagecustomizerapi.Verity) string {
	return verityDevicePathFromName(verity.Name)
}

func verityDevicePathFromName(name string) string {
	return imagecustomizerapi.DeviceMapperPath + "/" + name
}

func verityIdToPartition(configDeviceId string, deviceId *imagecustomizerapi.IdentifiedPartition,
	partIdToPartUuid map[string]string, diskPartitions []diskutils.PartitionInfo,
) (diskutils.PartitionInfo, error) {
	var partition diskutils.PartitionInfo
	var found bool
	switch {
	case configDeviceId != "":
		partUuid := partIdToPartUuid[configDeviceId]
		partition, found = sliceutils.FindValueFunc(diskPartitions, func(partition diskutils.PartitionInfo) bool {
			return partition.PartUuid == partUuid
		})
		if !found {
			return diskutils.PartitionInfo{}, fmt.Errorf("no partition found with device Id (%s)", configDeviceId)
		}
		return partition, nil

	case deviceId != nil:
		partition, found = sliceutils.FindValueFunc(diskPartitions, func(partition diskutils.PartitionInfo) bool {
			switch deviceId.IdType {
			case imagecustomizerapi.IdentifiedPartitionTypePartLabel:
				return partition.PartLabel == deviceId.Id
			default:
				return false
			}
		})
		if !found {
			return diskutils.PartitionInfo{}, fmt.Errorf("partition not found (%s=%s)", deviceId.IdType, deviceId.Id)
		}
		return partition, nil

	default:
		return diskutils.PartitionInfo{}, fmt.Errorf("must provide config device ID or partition ID")
	}
}

// systemdFormatPartitionUuid formats the partition UUID based on the ID type following systemd dm-verity style.
func systemdFormatPartitionUuid(partUuid string, mountIdType imagecustomizerapi.MountIdentifierType,
	partitions []diskutils.PartitionInfo, buildDir string,
) (string, error) {
	partition, _, err := findPartition(imagecustomizerapi.MountIdentifierTypePartUuid, partUuid, partitions, buildDir)
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

func SystemdFormatCorruptionOption(corruptionOption imagecustomizerapi.CorruptionOption) (string, error) {
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

func parseSystemdVerityOptions(options string) (imagecustomizerapi.CorruptionOption, string, error) {
	corruptionOption := imagecustomizerapi.CorruptionOptionIoError
	var hashSigPath string

	optionValues := strings.Split(options, ",")
	for _, option := range optionValues {
		switch {
		case option == "":
			// Ignore empty string.

		case option == "ignore-corruption":
			corruptionOption = imagecustomizerapi.CorruptionOptionIgnore

		case option == "panic-on-corruption":
			corruptionOption = imagecustomizerapi.CorruptionOptionPanic

		case option == "restart-on-corruption":
			corruptionOption = imagecustomizerapi.CorruptionOptionRestart

		case strings.HasPrefix(option, "root-hash-signature="):
			hashSigPath = strings.TrimPrefix(option, "root-hash-signature=")

		default:
			return "", "", fmt.Errorf("unknown verity option (%s)", option)
		}
	}

	return corruptionOption, hashSigPath, nil
}

func validateVerityDependencies(imageChroot *safechroot.Chroot) error {
	// "device-mapper" is required for dm-verity support because it provides "dmsetup",
	// which Dracut needs to install the "dm" module (a dependency of "systemd-veritysetup").
	requiredRpms := []string{"device-mapper"}

	// Iterate over each required package and check if it's installed.
	for _, pkg := range requiredRpms {
		logger.Log.Debugf("Checking if package (%s) is installed", pkg)
		if !isPackageInstalled(imageChroot, pkg) {
			return fmt.Errorf("package (%s) is not installed:\nthe following packages must be installed to use Verity: %v", pkg, requiredRpms)
		}
	}

	return nil
}

func updateUkiKernelArgsForVerity(verityMetadata []verityDeviceMetadata,
	partitions []diskutils.PartitionInfo, buildDir string, bootUuid string,
) error {
	newArgs, err := constructVerityKernelCmdlineArgs(verityMetadata, partitions, buildDir, bootUuid)
	if err != nil {
		return fmt.Errorf("failed to generate verity kernel arguments:\n%w", err)
	}

	err = appendKernelArgsToUkiCmdlineFile(buildDir, newArgs)
	if err != nil {
		return fmt.Errorf("failed to append verity kernel arguments to UKI cmdline file:\n%w", err)
	}

	return nil
}

func validateVerityMountPaths(imageConnection *ImageConnection, config *imagecustomizerapi.Config,
	partUuidToFstabEntry map[string]diskutils.FstabEntry, baseImageVerityMetadata []verityDeviceMetadata,
) error {
	if config.Storage.VerityPartitionsType != imagecustomizerapi.VerityPartitionsUsesExisting {
		// Either:
		// - Verity is not being used OR
		// - Partitions were customized and the verity checks were done during the API validity checks.
		// Either way, nothing to do.
		return nil
	}

	partitions, err := diskutils.GetDiskPartitions(imageConnection.Loopback().DevicePath())
	if err != nil {
		return err
	}

	verityDeviceMountPoint := make(map[*imagecustomizerapi.Verity]*imagecustomizerapi.MountPoint)
	for i := range config.Storage.Verity {
		verity := &config.Storage.Verity[i]

		dataPartition, err := findIdentifiedPartition(partitions, *verity.DataDevice)
		if err != nil {
			return fmt.Errorf("verity (%s) data partition not found:\n%w", verity.Id, err)
		}

		hashPartition, err := findIdentifiedPartition(partitions, *verity.HashDevice)
		if err != nil {
			return fmt.Errorf("verity (%s) hash partition not found:\n%w", verity.Id, err)
		}

		err = ensurePartitionNotAlreadyInUse(dataPartition.PartUuid, baseImageVerityMetadata)
		if err != nil {
			return fmt.Errorf("verity (%s) data partition is invalid:\n%w", verity.Id, err)
		}

		err = ensurePartitionNotAlreadyInUse(hashPartition.PartUuid, baseImageVerityMetadata)
		if err != nil {
			return fmt.Errorf("verity (%s) hash partition is invalid:\n%w", verity.Id, err)
		}

		dataFstabEntry, found := partUuidToFstabEntry[dataPartition.PartUuid]
		if !found {
			return fmt.Errorf("verity's (%s) data partition's fstab entry not found", verity.Id)
		}

		_, found = partUuidToFstabEntry[hashPartition.PartUuid]
		if found {
			return fmt.Errorf("verity's (%s) hash partition cannot have an fstab entry", verity.Id)
		}

		if hashPartition.FileSystemType != "" {
			return fmt.Errorf("verity's (%s) hash partition cannot have a filesystem", verity.Id)
		}

		mountPoint := &imagecustomizerapi.MountPoint{
			Path:    dataFstabEntry.Target,
			Options: dataFstabEntry.Options,
		}
		verityDeviceMountPoint[verity] = mountPoint
	}

	err = imagecustomizerapi.ValidateVerityMounts(config.Storage.Verity, verityDeviceMountPoint)
	if err != nil {
		return err
	}

	return nil
}

// Check if the partition is already being used for something else.
func ensurePartitionNotAlreadyInUse(partUuid string, baseImageVerityMetadata []verityDeviceMetadata) error {
	for _, verityMetadata := range baseImageVerityMetadata {
		if partUuid == verityMetadata.dataPartUuid {
			return fmt.Errorf("partition already in use as existing verity device's (%s) data partition", verityMetadata.name)
		}

		if partUuid == verityMetadata.hashPartUuid {
			return fmt.Errorf("partition already in use as existing verity device's (%s) hash partition", verityMetadata.name)
		}
	}

	return nil
}

func findIdentifiedPartition(partitions []diskutils.PartitionInfo, ref imagecustomizerapi.IdentifiedPartition,
) (diskutils.PartitionInfo, error) {
	partition, found := sliceutils.FindValueFunc(partitions, func(partition diskutils.PartitionInfo) bool {
		switch ref.IdType {
		case imagecustomizerapi.IdentifiedPartitionTypePartLabel:
			return partition.PartLabel == ref.Id

		default:
			return false
		}
	})
	if !found {
		return diskutils.PartitionInfo{}, fmt.Errorf("partition not found (%s=%s)", ref.IdType, ref.Id)
	}
	return partition, nil
}
