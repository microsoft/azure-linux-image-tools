// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/imageconnection"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/resources"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safeloopback"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/safemount"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/shell"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// Verity operation errors
	ErrVerityPackageDependencyValidation = NewImageCustomizerError("Verity:PackageDependencyValidation", "failed to validate verity package dependencies")
	ErrVerityDracutModuleAdd             = NewImageCustomizerError("Verity:DracutModuleAdd", "failed to add verity dracut module")
	ErrVerityFstabUpdate                 = NewImageCustomizerError("Verity:FstabUpdate", "failed to update fstab for verity")
	ErrVerityGrubConfigPrepare           = NewImageCustomizerError("Verity:GrubConfigPrepare", "failed to prepare grub config for verity")
	ErrVerityHashSignatureSupport        = NewImageCustomizerError("Verity:HashSignatureSupport", "failed to add verity hash signature support")
	ErrVerityFstabRead                   = NewImageCustomizerError("Verity:FstabRead", "failed to read fstab")
	ErrVerityImageConnection             = NewImageCustomizerError("Verity:ConnectToImage", "failed to connect to image file to provision verity")
	ErrGetDiskSectorSize                 = NewImageCustomizerError("Verity:GetSectorSize", "failed to get disk sector size")
	ErrMountPartition                    = NewImageCustomizerError("Verity:MountPartition", "failed to mount partition")
	ErrUpdateDisk                        = NewImageCustomizerError("Verity:UpdateDisk", "failed to wait for disk to update")
	ErrFindVerityDataPartition           = NewImageCustomizerError("Verity:FindDataPartition", "failed to find verity data partition")
	ErrFindVerityHashPartition           = NewImageCustomizerError("Verity:FindHashPartition", "failed to find verity hash partition")
	ErrCalculateRootHash                 = NewImageCustomizerError("Verity:CalculateRootHash", "failed to calculate root hash")
	ErrCompileRootHashRegex              = NewImageCustomizerError("Verity:CompileRootHashRegex", "failed to compile root hash regex")
	ErrParseRootHash                     = NewImageCustomizerError("Verity:ParseRootHash", "failed to parse root hash from veritysetup output")
	ErrCalculateHashSize                 = NewImageCustomizerError("Verity:CalculateHashSize", "failed to calculate hash partition size")
	ErrShrinkHashPartition               = NewImageCustomizerError("Verity:ShrinkHashPartition", "failed to shrink hash partition")
	ErrVerifyVerity                      = NewImageCustomizerError("Verity:Verify", "failed to verify verity")
	ErrUpdateKernelArgs                  = NewImageCustomizerError("Verity:UpdateKernelArgs", "failed to update kernel cmdline arguments for verity")
	ErrUpdateGrubConfig                  = NewImageCustomizerError("Verity:UpdateGrubConfig", "failed to update grub config for verity")
)

const (
	systemdVerityDracutModule = "systemd-veritysetup"
	dmVerityDracutDriver      = "dm-verity"
	mountBootPartModule       = "mountbootpartition"

	// Dracut module directory path for verity boot partition support.
	VerityMountBootPartitionModuleDir = "/usr/lib/dracut/modules.d/90mountbootpartition"
	// Standard permission mode for dracut module directories.
	DracutModuleDirMode = 0o755
	// Standard permission mode for executable scripts in dracut modules.
	DracutModuleScriptFileMode = 0o755
)

func enableVerityPartition(ctx context.Context, verity []imagecustomizerapi.Verity,
	imageChroot *safechroot.Chroot, distroHandler distroHandler,
) (bool, error) {
	var err error

	if len(verity) <= 0 {
		return false, nil
	}

	logger.Log.Infof("Enable verity")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "enable_verity_partition")
	defer span.End()

	err = validateVerityDependencies(imageChroot, distroHandler)
	if err != nil {
		return false, fmt.Errorf("%w:\n%w", ErrVerityPackageDependencyValidation, err)
	}

	// Integrate systemd veritysetup dracut module into initramfs img.
	err = addDracutModuleAndDriver(systemdVerityDracutModule, dmVerityDracutDriver, imageChroot)
	if err != nil {
		return false, fmt.Errorf("%w:\n%w", ErrVerityDracutModuleAdd, err)
	}

	err = updateFstabForVerity(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("%w:\n%w", ErrVerityFstabUpdate, err)
	}

	err = prepareGrubConfigForVerity(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("%w:\n%w", ErrVerityGrubConfigPrepare, err)
	}

	err = supportVerityHashSignature(verity, imageChroot)
	if err != nil {
		return false, fmt.Errorf("%w:\n%w", ErrVerityHashSignatureSupport, err)
	}

	return true, nil
}

func updateFstabForVerity(verityList []imagecustomizerapi.Verity, imageChroot *safechroot.Chroot) error {
	fstabFile := filepath.Join(imageChroot.RootDir(), "etc", "fstab")
	fstabEntries, err := diskutils.ReadFstabFile(fstabFile)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrVerityFstabRead, err)
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
			return fmt.Errorf("failed to install verity dracut file (%s):\n%w", dst, err)
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

func validateVerityDependencies(imageChroot *safechroot.Chroot, distroHandler distroHandler) error {
	// "device-mapper" is required for dm-verity support because it provides "dmsetup",
	// which Dracut needs to install the "dm" module (a dependency of "systemd-veritysetup").
	requiredRpms := []string{"device-mapper"}

	// Iterate over each required package and check if it's installed.
	for _, pkg := range requiredRpms {
		logger.Log.Debugf("Checking if package (%s) is installed", pkg)
		installed := distroHandler.isPackageInstalled(imageChroot, pkg)
		if !installed {
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

func validateVerityMountPaths(imageConnection *imageconnection.ImageConnection, config *imagecustomizerapi.Config,
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

func customizeVerityImageHelper(ctx context.Context, buildDir string, config *imagecustomizerapi.Config,
	buildImageFile string, partIdToPartUuid map[string]string, shrinkHashPartition bool,
	baseImageVerity []verityDeviceMetadata, readonlyPartUuids []string,
) ([]verityDeviceMetadata, error) {
	logger.Log.Infof("Provisioning verity")

	_, span := otel.GetTracerProvider().Tracer(OtelTracerName).Start(ctx, "provision_verity")
	defer span.End()

	verityMetadata := []verityDeviceMetadata(nil)

	loopback, err := safeloopback.NewLoopback(buildImageFile)
	if err != nil {
		return nil, fmt.Errorf("%w:\n%w", ErrVerityImageConnection, err)
	}
	defer loopback.Close()

	diskPartitions, err := diskutils.GetDiskPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	sectorSize, _, err := diskutils.GetSectorSize(loopback.DevicePath())
	if err != nil {
		return nil, fmt.Errorf("%w (device='%s'):\n%w", ErrGetDiskSectorSize, loopback.DevicePath(), err)
	}

	verityUpdated := false

	for _, metadata := range baseImageVerity {
		newMetadata := metadata

		readonly := slices.Contains(readonlyPartUuids, metadata.dataPartUuid)
		if !readonly {
			// Find partitions.
			dataPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
				metadata.dataPartUuid, diskPartitions)
			if err != nil {
				return nil, fmt.Errorf("%w (name='%s'):\n%w", ErrFindVerityDataPartition, metadata.name, err)
			}

			hashPartition, _, err := findPartitionHelper(imagecustomizerapi.MountIdentifierTypePartUuid,
				metadata.hashPartUuid, diskPartitions)
			if err != nil {
				return nil, fmt.Errorf("%w (name='%s'):\n%w", ErrFindVerityHashPartition, metadata.name, err)
			}

			// Format hash partition.
			rootHash, err := verityFormat(loopback.DevicePath(), dataPartition.Path, hashPartition.Path,
				shrinkHashPartition, sectorSize)
			if err != nil {
				return nil, err
			}

			newMetadata.rootHash = rootHash
			verityUpdated = true
		}

		verityMetadata = append(verityMetadata, newMetadata)
	}

	for _, verityConfig := range config.Storage.Verity {
		// Extract the partition block device path.
		dataPartition, err := verityIdToPartition(verityConfig.DataDeviceId, verityConfig.DataDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("%w (id='%s'):\n%w", ErrFindVerityDataPartition, verityConfig.Id, err)
		}
		hashPartition, err := verityIdToPartition(verityConfig.HashDeviceId, verityConfig.HashDevice, partIdToPartUuid,
			diskPartitions)
		if err != nil {
			return nil, fmt.Errorf("%w (id='%s'):\n%w", ErrFindVerityHashPartition, verityConfig.Id, err)
		}

		// Format hash partition.
		rootHash, err := verityFormat(loopback.DevicePath(), dataPartition.Path, hashPartition.Path,
			shrinkHashPartition, sectorSize)
		if err != nil {
			return nil, err
		}

		metadata := verityDeviceMetadata{
			name:                  verityConfig.Name,
			rootHash:              rootHash,
			dataPartUuid:          dataPartition.PartUuid,
			hashPartUuid:          hashPartition.PartUuid,
			dataDeviceMountIdType: verityConfig.DataDeviceMountIdType,
			hashDeviceMountIdType: verityConfig.HashDeviceMountIdType,
			corruptionOption:      verityConfig.CorruptionOption,
			hashSignaturePath:     verityConfig.HashSignaturePath,
		}
		verityMetadata = append(verityMetadata, metadata)
		verityUpdated = true
	}

	// Refresh disk partitions after running veritysetup so that the hash partition's UUID is correct.
	err = diskutils.RefreshPartitions(loopback.DevicePath())
	if err != nil {
		return nil, err
	}

	if verityUpdated {
		diskPartitions, err = diskutils.GetDiskPartitions(loopback.DevicePath())
		if err != nil {
			return nil, err
		}

		// Update kernel args.
		isUki := config.OS.Uki != nil
		err = updateKernelArgsForVerity(buildDir, diskPartitions, verityMetadata, isUki)
		if err != nil {
			return nil, err
		}
	}

	err = loopback.CleanClose()
	if err != nil {
		return nil, err
	}

	deviceNamesJson := getVerityNames(verityMetadata)
	span.SetAttributes(
		attribute.Int("verity_count", len(verityMetadata)),
		attribute.StringSlice("verity_device_name", deviceNamesJson),
	)

	return verityMetadata, nil
}

func verityFormat(diskDevicePath string, dataPartitionPath string, hashPartitionPath string, shrinkHashPartition bool,
	sectorSize uint64,
) (string, error) {
	// Write hash partition.
	verityOutput, _, err := shell.NewExecBuilder("veritysetup", "format", dataPartitionPath, hashPartitionPath).
		LogLevel(logrus.DebugLevel, logrus.DebugLevel).
		ErrorStderrLines(1).
		ExecuteCaptureOutput()
	if err != nil {
		return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrCalculateRootHash, dataPartitionPath, err)
	}

	// Extract root hash using regular expressions.
	rootHashRegex, err := regexp.Compile(`Root hash:\s+([0-9a-fA-F]+)`)
	if err != nil {
		return "", fmt.Errorf("%w:\n%w", ErrCompileRootHashRegex, err)
	}

	rootHashMatches := rootHashRegex.FindStringSubmatch(verityOutput)
	if len(rootHashMatches) <= 1 {
		return "", ErrParseRootHash
	}

	rootHash := rootHashMatches[1]

	err = diskutils.RefreshPartitions(diskDevicePath)
	if err != nil {
		return "", fmt.Errorf("%w (device='%s'):\n%w", ErrUpdateDisk, diskDevicePath, err)
	}

	if shrinkHashPartition {
		// Calculate the size of the hash partition from it's superblock.
		// In newer `veritysetup` versions, `veritysetup format` returns the size in its output. But that feature
		// is too new for now.
		hashPartitionSizeInBytes, err := calculateHashFileSizeInBytes(hashPartitionPath)
		if err != nil {
			return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrCalculateHashSize, hashPartitionPath, err)
		}

		hashPartitionSizeInSectors := convertBytesToSectors(hashPartitionSizeInBytes, sectorSize)

		err = resizePartition(hashPartitionPath, diskDevicePath, hashPartitionSizeInSectors)
		if err != nil {
			return "", fmt.Errorf("%w (device='%s'):\n%w", ErrShrinkHashPartition, diskDevicePath, err)
		}

		// Verify everything is still valid.
		err = shell.NewExecBuilder("veritysetup", "verify", dataPartitionPath, hashPartitionPath, rootHash).
			LogLevel(logrus.DebugLevel, logrus.DebugLevel).
			Execute()
		if err != nil {
			return "", fmt.Errorf("%w (partition='%s'):\n%w", ErrVerifyVerity, dataPartitionPath, err)
		}
	}

	return rootHash, nil
}

func updateKernelArgsForVerity(buildDir string, diskPartitions []diskutils.PartitionInfo,
	verityMetadata []verityDeviceMetadata, isUki bool,
) error {
	systemBootPartition, err := findSystemBootPartition(diskPartitions)
	if err != nil {
		return err
	}

	bootPartition, err := findBootPartitionFromEsp(systemBootPartition, diskPartitions, buildDir)
	if err != nil {
		return err
	}

	bootPartitionTmpDir := filepath.Join(buildDir, tmpBootPartitionDirName)
	// Temporarily mount the partition.
	bootPartitionMount, err := safemount.NewMount(bootPartition.Path, bootPartitionTmpDir, bootPartition.FileSystemType, 0, "", true)
	if err != nil {
		return fmt.Errorf("%w (partition='%s'):\n%w", ErrMountPartition, bootPartition.Path, err)
	}
	defer bootPartitionMount.Close()

	grubCfgFullPath := filepath.Join(bootPartitionTmpDir, DefaultGrubCfgPath)
	_, err = os.Stat(grubCfgFullPath)
	if err != nil {
		return fmt.Errorf("%w (file='%s'):\n%w", ErrStatFile, grubCfgFullPath, err)
	}

	if isUki {
		// UKI is enabled, update kernel cmdline args file.
		err = updateUkiKernelArgsForVerity(verityMetadata, diskPartitions, buildDir, bootPartition.Uuid)
		if err != nil {
			return fmt.Errorf("%w:\n%w", ErrUpdateKernelArgs, err)
		}
	}

	// Temporarily always update grub.cfg for verity, even when UKI is used.
	// Since grub dependencies are still kept under /boot and won't be cleaned.
	// This will be decoupled once the bootloader project is in place.
	err = updateGrubConfigForVerity(verityMetadata, grubCfgFullPath, diskPartitions, buildDir, bootPartition.Uuid)
	if err != nil {
		return fmt.Errorf("%w:\n%w", ErrUpdateGrubConfig, err)
	}

	err = bootPartitionMount.CleanClose()
	if err != nil {
		return err
	}

	return nil
}
