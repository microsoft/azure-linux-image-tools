// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"fmt"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
)

var (
	// Type conversion errors
	ErrBootTypeInvalid            = NewImageCustomizerError("TypeConversion:BootTypeInvalid", "invalid BootType value")
	ErrDiskSizeInvalid            = NewImageCustomizerError("TypeConversion:DiskSizeInvalid", "disk size must be multiple of 1 MiB")
	ErrPartitionTableTypeUnknown  = NewImageCustomizerError("TypeConversion:PartitionTableTypeUnknown", "unknown partition table type")
	ErrPartitionStartInvalid      = NewImageCustomizerError("TypeConversion:PartitionStartInvalid", "partition start must be multiple of 1 MiB")
	ErrPartitionEndInvalid        = NewImageCustomizerError("TypeConversion:PartitionEndInvalid", "partition end must be multiple of 1 MiB")
	ErrMountIdentifierTypeUnknown = NewImageCustomizerError("TypeConversion:MountIdentifierTypeUnknown", "unknown MountIdentifierType value")
	ErrSelinuxModeUnknown         = NewImageCustomizerError("TypeConversion:SelinuxModeUnknown", "unknown SELinuxMode value")
)

func bootTypeToImager(bootType imagecustomizerapi.BootType) (string, error) {
	switch bootType {
	case imagecustomizerapi.BootTypeEfi:
		return "efi", nil

	case imagecustomizerapi.BootTypeLegacy:
		return "legacy", nil

	default:
		return "", fmt.Errorf("%w (bootType='%s')", ErrBootTypeInvalid, bootType)
	}
}

func diskConfigToImager(diskConfig imagecustomizerapi.Disk, fileSystems []imagecustomizerapi.FileSystem,
) (configuration.Disk, error) {
	imagerPartitionTableType, err := partitionTableTypeToImager(diskConfig.PartitionTableType)
	if err != nil {
		return configuration.Disk{}, err
	}

	imagerPartitions, err := partitionsToImager(diskConfig.Partitions, fileSystems)
	if err != nil {
		return configuration.Disk{}, err
	}

	imagerMaxSize := *diskConfig.MaxSize / diskutils.MiB
	if *diskConfig.MaxSize%diskutils.MiB != 0 {
		return configuration.Disk{}, fmt.Errorf("%w (size='%d')", ErrDiskSizeInvalid, *diskConfig.MaxSize)
	}

	imagerDisk := configuration.Disk{
		PartitionTableType: imagerPartitionTableType,
		MaxSize:            uint64(imagerMaxSize),
		Partitions:         imagerPartitions,
	}
	return imagerDisk, err
}

func partitionTableTypeToImager(partitionTableType imagecustomizerapi.PartitionTableType,
) (configuration.PartitionTableType, error) {
	switch partitionTableType {
	case imagecustomizerapi.PartitionTableTypeGpt:
		return configuration.PartitionTableTypeGpt, nil

	default:
		return "", fmt.Errorf("%w (type='%s')", ErrPartitionTableTypeUnknown, partitionTableType)
	}
}

func partitionsToImager(partitions []imagecustomizerapi.Partition, fileSystems []imagecustomizerapi.FileSystem,
) ([]configuration.Partition, error) {
	imagerPartitions := []configuration.Partition(nil)
	for _, partition := range partitions {
		imagerPartition, err := partitionToImager(partition, fileSystems)
		if err != nil {
			return nil, err
		}

		imagerPartitions = append(imagerPartitions, imagerPartition)
	}

	return imagerPartitions, nil
}

func partitionToImager(partition imagecustomizerapi.Partition, fileSystems []imagecustomizerapi.FileSystem,
) (configuration.Partition, error) {
	fileSystem, _ := sliceutils.FindValueFunc(fileSystems,
		func(fileSystem imagecustomizerapi.FileSystem) bool {
			return fileSystem.PartitionId == partition.Id
		},
	)

	imagerStart := *partition.Start / diskutils.MiB
	if *partition.Start%diskutils.MiB != 0 {
		return configuration.Partition{}, fmt.Errorf("%w (start='%d')", ErrPartitionStartInvalid, *partition.Start)
	}

	end, _ := partition.GetEnd()
	imagerEnd := end / diskutils.MiB
	if end%diskutils.MiB != 0 {
		return configuration.Partition{}, fmt.Errorf("%w (end='%d')", ErrPartitionEndInvalid, end)
	}

	imagerFlags, typeUuid, err := toImagerPartitionFlags(partition.Type)
	if err != nil {
		return configuration.Partition{}, err
	}

	imagerPartition := configuration.Partition{
		ID:       partition.Id,
		FsType:   string(fileSystem.Type),
		Name:     partition.Label,
		Start:    uint64(imagerStart),
		End:      uint64(imagerEnd),
		Flags:    imagerFlags,
		TypeUUID: typeUuid,
	}
	return imagerPartition, nil
}

func toImagerPartitionFlags(partitionType imagecustomizerapi.PartitionType,
) ([]configuration.PartitionFlag, string, error) {
	switch partitionType {
	case imagecustomizerapi.PartitionTypeESP:
		return []configuration.PartitionFlag{configuration.PartitionFlagESP, configuration.PartitionFlagBoot}, "", nil

	case imagecustomizerapi.PartitionTypeBiosGrub:
		return []configuration.PartitionFlag{configuration.PartitionFlagBiosGrub}, "", nil

	default:
		typeUuid, foundTypeUuid := imagecustomizerapi.PartitionTypeToUuid[partitionType]
		if !foundTypeUuid {
			// If an entry is not found in PartitionTypeToUuid, then the partitionType must be a UUID string.
			typeUuid = string(partitionType)
		}

		return nil, typeUuid, nil
	}
}

func partitionSettingsToImager(fileSystems []imagecustomizerapi.FileSystem,
) ([]configuration.PartitionSetting, error) {
	imagerPartitionSettings := []configuration.PartitionSetting(nil)
	for _, fileSystem := range fileSystems {
		imagerPartitionSetting, err := partitionSettingToImager(fileSystem)
		if err != nil {
			return nil, err
		}
		imagerPartitionSettings = append(imagerPartitionSettings, imagerPartitionSetting)
	}
	return imagerPartitionSettings, nil
}

func partitionSettingToImager(fileSystem imagecustomizerapi.FileSystem,
) (configuration.PartitionSetting, error) {
	mountIdType := imagecustomizerapi.MountIdentifierTypeDefault
	mountOptions := ""
	mountPath := ""
	if fileSystem.MountPoint != nil {
		mountIdType = fileSystem.MountPoint.IdType
		mountOptions = fileSystem.MountPoint.Options
		mountPath = fileSystem.MountPoint.Path
	}

	imagerMountIdentifierType, err := mountIdentifierTypeToImager(mountIdType)
	if err != nil {
		return configuration.PartitionSetting{}, err
	}

	imagerPartitionSetting := configuration.PartitionSetting{
		ID:              fileSystem.PartitionId,
		MountIdentifier: imagerMountIdentifierType,
		MountOptions:    mountOptions,
		MountPoint:      mountPath,
	}
	return imagerPartitionSetting, nil
}

func mountIdentifierTypeToImager(mountIdentifierType imagecustomizerapi.MountIdentifierType,
) (configuration.MountIdentifier, error) {
	switch mountIdentifierType {
	case imagecustomizerapi.MountIdentifierTypeUuid:
		return configuration.MountIdentifierUuid, nil

	case imagecustomizerapi.MountIdentifierTypePartUuid, imagecustomizerapi.MountIdentifierTypeDefault:
		return configuration.MountIdentifierPartUuid, nil

	case imagecustomizerapi.MountIdentifierTypePartLabel:
		return configuration.MountIdentifierPartLabel, nil

	default:
		return "", fmt.Errorf("%w (type='%s')", ErrMountIdentifierTypeUnknown, mountIdentifierType)
	}
}

func kernelCommandLineToImager(kernelCommandLine imagecustomizerapi.KernelCommandLine,
	selinuxConfig imagecustomizerapi.SELinux,
	currentSELinuxMode imagecustomizerapi.SELinuxMode,
) (configuration.KernelCommandLine, error) {
	imagerSELinuxMode, err := selinuxModeMaybeDefaultToImager(selinuxConfig.Mode, currentSELinuxMode)
	if err != nil {
		return configuration.KernelCommandLine{}, err
	}

	imagerKernelCommandLine := configuration.KernelCommandLine{
		ExtraCommandLine: GrubArgsToString(kernelCommandLine.ExtraCommandLine),
		SELinux:          imagerSELinuxMode,
		SELinuxPolicy:    "",
	}
	return imagerKernelCommandLine, nil
}

func selinuxModeMaybeDefaultToImager(selinuxMode imagecustomizerapi.SELinuxMode,
	currentSELinuxMode imagecustomizerapi.SELinuxMode,
) (configuration.SELinux, error) {
	if selinuxMode == imagecustomizerapi.SELinuxModeDefault {
		selinuxMode = currentSELinuxMode
	}

	return selinuxModeToImager(selinuxMode)
}

func selinuxModeToImager(selinuxMode imagecustomizerapi.SELinuxMode) (configuration.SELinux, error) {
	switch selinuxMode {
	case imagecustomizerapi.SELinuxModeDisabled:
		return configuration.SELinuxOff, nil

	case imagecustomizerapi.SELinuxModeEnforcing:
		return configuration.SELinuxEnforcing, nil

	case imagecustomizerapi.SELinuxModePermissive:
		return configuration.SELinuxPermissive, nil

	case imagecustomizerapi.SELinuxModeForceEnforcing:
		return configuration.SELinuxForceEnforcing, nil

	default:
		return "", fmt.Errorf("%w (mode='%s')", ErrSelinuxModeUnknown, selinuxMode)
	}
}
