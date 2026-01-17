// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/sliceutils"
)

type VerityPartitionsType int

const (
	VerityPartitionsInvalid VerityPartitionsType = iota
	VerityPartitionsNone
	VerityPartitionsUsesConfig
	VerityPartitionsUsesExisting
)

type Storage struct {
	ResetPartitionsUuidsType ResetPartitionsUuidsType `yaml:"resetPartitionsUuidsType" json:"resetPartitionsUuidsType,omitempty"`
	BootType                 BootType                 `yaml:"bootType" json:"bootType,omitempty"`
	Disks                    []Disk                   `yaml:"disks" json:"disks,omitempty"`
	FileSystems              []FileSystem             `yaml:"filesystems" json:"filesystems,omitempty"`
	Verity                   []Verity                 `yaml:"verity" json:"verity,omitempty"`
	ReinitializeVerity       ReinitializeVerityType   `yaml:"reinitializeVerity" json:"reinitializeVerity,omitempty"`

	// Filled in by Storage.IsValid().
	VerityPartitionsType VerityPartitionsType `json:"-"`
}

func (s *Storage) IsValid() error {
	var err error

	err = s.ResetPartitionsUuidsType.IsValid()
	if err != nil {
		return err
	}

	err = s.BootType.IsValid()
	if err != nil {
		return err
	}

	if len(s.Disks) > 1 {
		return fmt.Errorf("defining multiple disks is not currently supported")
	}

	for i := range s.Disks {
		disk := &s.Disks[i]

		err := disk.IsValid()
		if err != nil {
			return fmt.Errorf("invalid disk at index %d:\n%w", i, err)
		}
	}

	verityUsesConfigPartitions := false
	verityUsesExistingPartitions := false
	for i, verity := range s.Verity {
		err = verity.IsValid()
		if err != nil {
			return fmt.Errorf("invalid verity item at index %d:\n%w", i, err)
		}

		if verity.DataDeviceId != "" || verity.HashDeviceId != "" {
			verityUsesConfigPartitions = true
		}

		if verity.DataDevice != nil || verity.HashDevice != nil {
			verityUsesExistingPartitions = true
		}
	}

	s.VerityPartitionsType = VerityPartitionsInvalid
	switch {
	case verityUsesConfigPartitions && verityUsesExistingPartitions:
		return fmt.Errorf("cannot use both dataDeviceId/hashDeviceId and dataDevice/hashDevice")

	case verityUsesConfigPartitions:
		s.VerityPartitionsType = VerityPartitionsUsesConfig

	case verityUsesExistingPartitions:
		s.VerityPartitionsType = VerityPartitionsUsesExisting

	default:
		s.VerityPartitionsType = VerityPartitionsNone
	}

	for i, fileSystem := range s.FileSystems {
		err = fileSystem.IsValid()
		if err != nil {
			return fmt.Errorf("invalid filesystems item at index %d:\n%w", i, err)
		}
	}

	err = s.ReinitializeVerity.IsValid()
	if err != nil {
		return fmt.Errorf("invalid 'reinitializeVerity' value:\n%w", err)
	}

	hasResetUuids := s.ResetPartitionsUuidsType != ResetPartitionsUuidsTypeDefault
	hasBootType := s.BootType != BootTypeNone
	hasDisks := len(s.Disks) > 0
	hasFileSystems := len(s.FileSystems) > 0

	if hasResetUuids && hasDisks {
		return fmt.Errorf("cannot specify both 'resetPartitionsUuidsType' and 'disks'")
	}

	if !hasBootType && hasDisks {
		return fmt.Errorf("must specify 'bootType' if 'disks' are specified")
	}

	if hasBootType && !hasDisks {
		return fmt.Errorf("cannot specify 'bootType' without specifying 'disks'")
	}

	if hasFileSystems && !hasDisks {
		return fmt.Errorf("cannot specify 'filesystems' without specifying 'disks'")
	}

	if s.VerityPartitionsType == VerityPartitionsUsesConfig && !hasDisks {
		return fmt.Errorf("cannot specify 'verity' with dataDeviceId/hashDeviceId without specifying 'disks'")
	}

	if s.VerityPartitionsType == VerityPartitionsUsesExisting && hasDisks {
		return fmt.Errorf("cannot specify both 'verity' with dataDevice/hashDevice and 'disks'")
	}

	// Create a set of all block devices by their Id.
	deviceMap, partitionLabelCounts, err := s.buildDeviceMap()
	if err != nil {
		return err
	}

	// Check that all child block devices exist and are not used by multiple things.
	deviceParents, err := s.checkDeviceTree(deviceMap, partitionLabelCounts)
	if err != nil {
		return err
	}

	espPartitionExists := false
	biosBootPartitionExists := false

	for _, disk := range s.Disks {
		for _, partition := range disk.Partitions {
			fileSystem, hasFileSystem := deviceParents[partition.Id].(*FileSystem)

			// Ensure special partitions have the correct filesystem type.
			switch partition.Type {
			case PartitionTypeESP:
				espPartitionExists = true

				if !hasFileSystem || (fileSystem.Type != FileSystemTypeFat32 && fileSystem.Type != FileSystemTypeVfat) {
					return fmt.Errorf("ESP partition (%s) must have 'fat32' or 'vfat' filesystem type", partition.Id)
				}

				if fileSystem.MountPoint == nil || fileSystem.MountPoint.Path != "/boot/efi" {
					return fmt.Errorf("ESP partition (%s) must be mounted at /boot/efi", partition.Id)
				}

			case PartitionTypeBiosGrub:
				biosBootPartitionExists = true

				if hasFileSystem {
					if fileSystem.Type != FileSystemTypeNone {
						return fmt.Errorf("BIOS boot partition (%s) must not have a filesystem 'type'",
							partition.Id)
					}

					if fileSystem.MountPoint != nil {
						return fmt.Errorf("BIOS boot partition (%s) must not have a 'mountPoint'", partition.Id)
					}
				}

			default:
				if hasFileSystem {
					expectedMountPaths, hasExpectedMountPaths := PartitionTypeSupportedMountPaths[partition.Type]
					if hasExpectedMountPaths {
						// BTRFS subvolume mounts are not supported for partition types with expected mount paths.
						if fileSystem.Btrfs != nil {
							for _, subvolume := range fileSystem.Btrfs.Subvolumes {
								if subvolume.MountPoint != nil {
									logger.Log.Infof(
										"partition (%s) with type (%s) does not support BTRFS subvolume mounts",
										partition.Id, partition.Type)
									break
								}
							}
						}

						if fileSystem.MountPoint != nil {
							supportedPath := sliceutils.ContainsValue(expectedMountPaths, fileSystem.MountPoint.Path)
							if !supportedPath {
								logger.Log.Infof(
									"Unexpected mount path (%s) for partition (%s) with type (%s). Expected paths: %v",
									fileSystem.MountPoint.Path, partition.Id, partition.Type, expectedMountPaths)
							}
						}
					}
				}
			}
		}
	}

	// Ensure the correct partitions exist to support the specified the boot type.
	switch s.BootType {
	case BootTypeEfi:
		if !espPartitionExists {
			return fmt.Errorf("'esp' partition must be provided for 'efi' boot type")
		}

	case BootTypeLegacy:
		if !biosBootPartitionExists {
			return fmt.Errorf("'bios-grub' partition must be provided for 'legacy' boot type")
		}
	}

	// Validate verity filesystem settings.
	if s.VerityPartitionsType == VerityPartitionsUsesConfig {
		verityDeviceMount := make(map[*Verity]*VerityMount)

		for i := range s.Verity {
			verity := &s.Verity[i]
			verityDeviceMounts := []*VerityMount{}

			filesystem, hasFileSystem := deviceParents[verity.Id].(*FileSystem)
			if hasFileSystem {
				if filesystem.MountPoint != nil {
					verityMount := &VerityMount{
						MountPath:     filesystem.MountPoint.Path,
						MountOptions:  filesystem.MountPoint.Options,
						SubvolumePath: "",
					}
					verityDeviceMounts = append(verityDeviceMounts, verityMount)
				}

				// Also collect mount points from BTRFS subvolumes
				if filesystem.Btrfs != nil {
					for j := range filesystem.Btrfs.Subvolumes {
						subvolume := &filesystem.Btrfs.Subvolumes[j]
						if subvolume.MountPoint != nil {
							verityMount := &VerityMount{
								MountPath:     subvolume.MountPoint.Path,
								MountOptions:  subvolume.MountPoint.Options,
								SubvolumePath: subvolume.Path,
							}
							verityDeviceMounts = append(verityDeviceMounts, verityMount)
						}
					}
				}
			}

			// The empty case is handled in ValidateVerityMounts() to consolidate error handling.
			if len(verityDeviceMounts) == 1 {
				verityDeviceMount[verity] = verityDeviceMounts[0]
			} else if len(verityDeviceMounts) > 1 {
				return fmt.Errorf("verity device (%s) has multiple mount points, which is not supported", verity.Id)
			}
		}

		err := ValidateVerityMounts(s.Verity, verityDeviceMount)
		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateVerityMounts(verityDevices []Verity, verityDeviceMount map[*Verity]*VerityMount) error {
	for i := range verityDevices {
		verity := &verityDevices[i]

		mount, hasMount := verityDeviceMount[verity]
		if !hasMount || (mount.MountPath != "/" && mount.MountPath != "/usr") {
			return fmt.Errorf("mount path of verity device (%s) must be set to '/' or '/usr'", verity.Id)
		}

		verity.Mount = *mount

		expectedVerityName, validMount := verityMountMap[mount.MountPath]
		if !validMount || verity.Name != expectedVerityName {
			return fmt.Errorf(
				"mount path of verity device (%s) must match verity name: '%s' for '%s'",
				verity.Id, expectedVerityName, mount.MountPath,
			)
		}

		mountOptions := strings.Split(mount.MountOptions, ",")
		if !sliceutils.ContainsValue(mountOptions, "ro") {
			return fmt.Errorf("verity device's (%s) mount must include the 'ro' mount option", verity.Id)
		}
	}

	return nil
}

func (s *Storage) CustomizePartitions() bool {
	return len(s.Disks) > 0
}

func (s *Storage) buildDeviceMap() (map[string]any, map[string]int, error) {
	deviceMap := make(map[string]any)
	partitionLabelCounts := make(map[string]int)

	for i, disk := range s.Disks {
		for j := range disk.Partitions {
			partition := &disk.Partitions[j]

			if _, existingName := deviceMap[partition.Id]; existingName {
				return nil, nil, fmt.Errorf("invalid disk at index %d:\ninvalid partition at index %d:\nduplicate id (%s)",
					i, j, partition.Id)
			}

			deviceMap[partition.Id] = partition

			// Count the number of partitions that use each label.
			partitionLabelCounts[partition.Label] += 1
		}
	}

	for i := range s.Verity {
		verity := &s.Verity[i]

		if _, existingName := deviceMap[verity.Id]; existingName {
			return nil, nil, fmt.Errorf("invalid verity item at index %d:\nduplicate id (%s)", i, verity.Id)
		}

		deviceMap[verity.Id] = verity
	}

	return deviceMap, partitionLabelCounts, nil
}

func (s *Storage) checkDeviceTree(deviceMap map[string]any, partitionLabelCounts map[string]int,
) (map[string]any, error) {
	deviceParents := make(map[string]any)

	for i := range s.Verity {
		verity := &s.Verity[i]

		err := checkDeviceTreeVerityItem(verity, deviceMap, deviceParents)
		if err != nil {
			return nil, fmt.Errorf("invalid verity item at index %d:\n%w", i, err)
		}
	}

	mountPaths := make(map[string]bool)
	for i := range s.FileSystems {
		filesystem := &s.FileSystems[i]

		err := checkDeviceTreeFileSystemItem(filesystem, deviceMap, deviceParents, partitionLabelCounts, mountPaths)
		if err != nil {
			return nil, fmt.Errorf("invalid filesystem item at index %d:\n%w", i, err)
		}
	}

	return deviceParents, nil
}

func checkDeviceTreeVerityItem(verity *Verity, deviceMap map[string]any, deviceParents map[string]any) error {
	if verity.DataDeviceId != "" {
		err := addVerityParentToDevice(verity.DataDeviceId, deviceMap, deviceParents, verity)
		if err != nil {
			return fmt.Errorf("invalid 'dataDeviceId':\n%w", err)
		}
	}

	if verity.HashDeviceId != "" {
		err := addVerityParentToDevice(verity.HashDeviceId, deviceMap, deviceParents, verity)
		if err != nil {
			return fmt.Errorf("invalid 'hashDeviceId':\n%w", err)
		}
	}

	return nil
}

func addVerityParentToDevice(deviceId string, deviceMap map[string]any, deviceParents map[string]any, parent *Verity,
) error {
	device, err := addParentToDevice(deviceId, deviceMap, deviceParents, parent)
	if err != nil {
		return err
	}

	switch device.(type) {
	case *Partition:

	default:
		return fmt.Errorf("device (%s) must be a partition", deviceId)
	}

	return nil
}

func checkDeviceTreeFileSystemItem(filesystem *FileSystem, deviceMap map[string]any, deviceParents map[string]any,
	partitionLabelCounts map[string]int, mountPaths map[string]bool,
) error {
	device, err := addParentToDevice(filesystem.DeviceId, deviceMap, deviceParents, filesystem)
	if err != nil {
		return fmt.Errorf("invalid 'deviceId':\n%w", err)
	}

	// Check for a duplicate mount path on the filesystem itself.
	if filesystem.MountPoint != nil {
		if _, existingMountPath := mountPaths[filesystem.MountPoint.Path]; existingMountPath {
			return fmt.Errorf("duplicate 'mountPoint.path' (%s)", filesystem.MountPoint.Path)
		}

		mountPaths[filesystem.MountPoint.Path] = true
	}

	// Check for duplicate mount paths in BTRFS subvolumes.
	if filesystem.Btrfs != nil {
		for _, subvolume := range filesystem.Btrfs.Subvolumes {
			if subvolume.MountPoint != nil {
				if _, existingMountPath := mountPaths[subvolume.MountPoint.Path]; existingMountPath {
					return fmt.Errorf("duplicate 'mountPoint.path' (%s) in btrfs subvolume (%s)",
						subvolume.MountPoint.Path, subvolume.Path)
				}
				mountPaths[subvolume.MountPoint.Path] = true
			}
		}
	}

	switch device := device.(type) {
	case *Partition:
		filesystem.PartitionId = filesystem.DeviceId
		checkDeviceLabelCount := false

		// Check partition label requirements for main mount point
		if filesystem.MountPoint != nil && filesystem.MountPoint.IdType == MountIdentifierTypePartLabel {
			checkDeviceLabelCount = true
			if device.Label == "" {
				return fmt.Errorf("idType set to 'part-label' but partition (%s) has no label set", device.Id)
			}
		}

		// Check partition label requirements for BTRFS subvolume mount points
		if filesystem.Btrfs != nil {
			for _, subvolume := range filesystem.Btrfs.Subvolumes {
				if subvolume.MountPoint != nil && subvolume.MountPoint.IdType == MountIdentifierTypePartLabel {
					checkDeviceLabelCount = true
					if device.Label == "" {
						return fmt.Errorf(
							"idType set to 'part-label' for btrfs subvolume (%s) but partition (%s) has no label set",
							subvolume.Path, device.Id)
					}
				}
			}
		}

		if checkDeviceLabelCount && partitionLabelCounts[device.Label] > 1 {
			return fmt.Errorf("more than one partition has a label of (%s)", device.Label)
		}

	case *Verity:
		filesystem.PartitionId = device.DataDeviceId

		// Check filesystem mount point for verity devices
		if filesystem.MountPoint != nil && filesystem.MountPoint.IdType != MountIdentifierTypeDefault {
			return fmt.Errorf("filesystem for verity device (%s) may not specify 'mountPoint.idType'",
				filesystem.DeviceId)
		}

		// Check BTRFS subvolume mount points for verity devices
		if filesystem.Btrfs != nil {
			for _, subvolume := range filesystem.Btrfs.Subvolumes {
				if subvolume.MountPoint != nil && subvolume.MountPoint.IdType != MountIdentifierTypeDefault {
					return fmt.Errorf(
						"btrfs subvolume (%s) for verity device (%s) may not specify 'mountPoint.idType'",
						subvolume.Path, filesystem.DeviceId)
				}
			}
		}

	default:

	}

	return nil
}

func addParentToDevice(deviceId string, deviceMap map[string]any, deviceParents map[string]any, parent any,
) (any, error) {
	device, deviceExists := deviceMap[deviceId]
	if !deviceExists {
		return nil, fmt.Errorf("device (%s) not found", deviceId)
	}

	if _, deviceInUse := deviceParents[deviceId]; deviceInUse {
		return nil, fmt.Errorf("device (%s) is used by multiple things", deviceId)
	}

	deviceParents[deviceId] = parent
	return device, nil
}
