// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// PartitionFlag describes the features of a partition
type PartitionFlag string

const (
	// PartitionFlagESP indicates this is the UEFI esp partition
	PartitionFlagESP PartitionFlag = "esp"
	// PartitionFlagGrub indicates this is a grub boot partition
	PartitionFlagGrub PartitionFlag = "grub"
	// PartitionFlagBiosGrub indicates this is a bios grub boot partition
	PartitionFlagBiosGrub PartitionFlag = "bios_grub"
	// PartitionFlagBiosGrubLegacy indicates this is a bios grub boot partition. Needed to preserve legacy config behavior.
	PartitionFlagBiosGrubLegacy PartitionFlag = "bios-grub"
	// PartitionFlagBoot indicates this is a boot partition
	PartitionFlagBoot PartitionFlag = "boot"
	// PartitionFlagDeviceMapperRoot indicates this partition will be used for a device mapper root device
	PartitionFlagDeviceMapperRoot PartitionFlag = "dmroot"
)
