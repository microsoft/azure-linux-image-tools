// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
)

// PartitionType describes the type of boot partition.
type PartitionType string

const (
	// PartitionTypeDefault indicates this is a normal partition.
	PartitionTypeDefault PartitionType = ""

	// PartitionTypeESP indicates this is a UEFI System Partition (ESP).
	PartitionTypeESP PartitionType = "esp"

	// PartitionTypeBiosGrub indicates this is the BIOS boot partition.
	// This is required for GPT disks that wish to be bootable using legacy BIOS mode.
	// This partition must start at block 1.
	//
	// See, https://en.wikipedia.org/wiki/BIOS_boot_partition
	PartitionTypeBiosGrub PartitionType = "bios-grub"

	PartitionTypeHome         = "home"
	PartitionTypeLinuxGeneric = "linux-generic"
	PartitionTypeRoot         = "root"
	PartitionTypeRootVerity   = "root-verity"
	PartitionTypeSrv          = "srv"
	PartitionTypeSwap         = "swap"
	PartitionTypeTmp          = "tmp"
	PartitionTypeUsr          = "usr"
	PartitionTypeUsrVerity    = "usr-verity"
	PartitionTypeVar          = "var"
	PartitionTypeXbootldr     = "xbootldr"
)

var (
	// UUIDs come from:
	// - https://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_type_GUIDs
	// - https://uapi-group.org/specifications/specs/discoverable_partitions_specification/
	partitionTypeToUuidArchIndependent = map[PartitionType]string{
		PartitionTypeESP:      "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
		PartitionTypeBiosGrub: "21686148-6449-6E6F-744E-656564454649",
		PartitionTypeHome:     "933ac7e1-2eb4-4f13-b844-0e14e2aef915",
		PartitionTypeSrv:      "3b8f8425-20e0-4f3b-907f-1a25a76f98e8",
		PartitionTypeSwap:     "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f",
		PartitionTypeTmp:      "7ec6f557-3bc5-4aca-b293-16ef5df639d1",
		PartitionTypeVar:      "4d21b016-b534-45c2-a9fb-5c16e091fd2d",
		PartitionTypeXbootldr: "bc13c2ff-59e6-4262-a352-b275fd6f7172",
	}

	PartitionTypeToUuid map[PartitionType]string

	// List of supported mount paths for each partition type.
	// No entry means there are no associated mount paths for the partition type.
	// Empty list means the partition type should not be mounted.
	PartitionTypeSupportedMountPaths = map[PartitionType][]string{
		PartitionTypeESP: {
			"/boot/efi",
		},
		PartitionTypeBiosGrub: {},
		PartitionTypeHome: {
			"/home",
		},
		PartitionTypeRoot: {
			"/",
		},
		PartitionTypeRootVerity: {},
		PartitionTypeSrv: {
			"/srv",
		},
		PartitionTypeSwap: {},
		PartitionTypeTmp: {
			"/var/tmp",
		},
		PartitionTypeUsr: {
			"/usr",
		},
		PartitionTypeUsrVerity: {},
		PartitionTypeVar: {
			"/var",
		},
		PartitionTypeXbootldr: {
			"/boot",
		},
	}
)

func init() {
	PartitionTypeToUuid = make(map[PartitionType]string)

	for k, v := range partitionTypeToUuidArchIndependent {
		PartitionTypeToUuid[k] = v
	}

	for k, v := range partitionTypeToUuidArchDependent {
		PartitionTypeToUuid[k] = v
	}
}

func (p PartitionType) IsValid() (err error) {
	switch p {
	case PartitionTypeDefault, PartitionTypeESP, PartitionTypeBiosGrub, PartitionTypeHome, PartitionTypeLinuxGeneric,
		PartitionTypeRoot, PartitionTypeRootVerity, PartitionTypeSrv, PartitionTypeSwap, PartitionTypeTmp,
		PartitionTypeUsr, PartitionTypeUsrVerity, PartitionTypeVar, PartitionTypeXbootldr:
		// String is a well known name.
		return nil

	default:
		isUuid := uuidRegex.MatchString(string(p))
		if isUuid {
			// String is a UUID.
			return nil
		}

		return fmt.Errorf("partition type is unknown and is not a UUID (%s)", p)
	}
}
