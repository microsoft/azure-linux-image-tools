// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"fmt"
	"regexp"
)

const (
	DeviceMapperPath = "/dev/mapper"

	VerityRootDeviceName = "root"
	VerityUsrDeviceName  = "usr"

	VerityRootDevicePath = DeviceMapperPath + "/" + VerityRootDeviceName
	VerityUsrDevicePath  = DeviceMapperPath + "/" + VerityUsrDeviceName
)

var (
	verityNameRegex = regexp.MustCompile("^[a-z]+$")

	VerityMountMap = map[string]string{
		"/":    VerityRootDeviceName,
		"/usr": VerityUsrDeviceName,
	}
)

type Verity struct {
	// ID is used to correlate `Verity` objects with `FileSystem` objects.
	Id string `yaml:"id" json:"id,omitempty"`
	// The name of the mapper block device.
	// Must be 'root' for the rootfs (/) filesystem.
	Name string `yaml:"name" json:"name,omitempty"`
	// The ID of the 'Partition' to use as the data partition.
	DataDeviceId string `yaml:"dataDeviceId" json:"dataDeviceId,omitempty"`
	// The partition to use as the data partition.
	// Mutually exclusive with 'DataDeviceId'.
	DataDevice *IdentifiedPartition `yaml:"dataDevice" json:"dataDevice,omitempty"`
	// The device ID type used to reference the data partition.
	DataDeviceMountIdType MountIdentifierType `yaml:"dataDeviceMountIdType" json:"dataDeviceMountIdType,omitempty"`
	// The ID of the 'Partition' to use as the hash partition.
	HashDeviceId string `yaml:"hashDeviceId" json:"hashDeviceId,omitempty"`
	// The partition to use as the hash partition.
	// Mutually exclusive with 'HashDeviceId'.
	HashDevice *IdentifiedPartition `yaml:"hashDevice" json:"hashDevice,omitempty"`
	// The device ID type used to reference the data partition.
	HashDeviceMountIdType MountIdentifierType `yaml:"hashDeviceMountIdType" json:"hashDeviceMountIdType,omitempty"`
	// How to handle corruption.
	CorruptionOption CorruptionOption `yaml:"corruptionOption" json:"corruptionOption,omitempty"`

	// Path to the root hash signature to inject into the image.
	HashSignatureInjection string `yaml:"hashSignatureInjection" json:"hashSignatureInjection,omitempty"`

	// The mount point of the verity device.
	// Value is filled in by ValidateVerityMounts() (via Storage.IsValid() or validateVerityMountPaths()).
	MountPath string `json:"-"`
}

func (v *Verity) IsValid() error {
	if v.Id == "" {
		return fmt.Errorf("'id' may not be empty")
	}

	if !verityNameRegex.MatchString(v.Name) {
		return fmt.Errorf("invalid 'name' value (%s)", v.Name)
	}

	if v.DataDeviceId == "" && v.DataDevice == nil {
		return fmt.Errorf("either 'dataDeviceId' or 'dataDevice' must be specified")
	}
	if v.DataDeviceId != "" && v.DataDevice != nil {
		return fmt.Errorf("cannot specify both 'dataDeviceId' and 'dataDevice'")
	}

	if v.HashDeviceId == "" && v.HashDevice == nil {
		return fmt.Errorf("either 'hashDeviceId' or 'hashDevice' must be specified")
	}
	if v.HashDeviceId != "" && v.HashDevice != nil {
		return fmt.Errorf("cannot specify both 'hashDeviceId' and 'hashDevice'")
	}

	usesConfigPartitions := v.HashDeviceId != "" || v.DataDeviceId != ""
	usesExistingPartitions := v.HashDevice != nil || v.DataDevice != nil

	if usesConfigPartitions && usesExistingPartitions {
		return fmt.Errorf("cannot use both dataDeviceId/hashDeviceId and dataDevice/hashDevice")
	}

	if err := v.CorruptionOption.IsValid(); err != nil {
		return fmt.Errorf("invalid corruptionOption:\n%w", err)
	}

	if v.HashSignatureInjection != "" {
		if err := validatePath(v.HashSignatureInjection); err != nil {
			return fmt.Errorf("invalid hashSignatureInjection path:\n%w", err)
		}
	}

	return nil
}
