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
)

var (
	verityNameRegex = regexp.MustCompile("^[a-z]+$")
)

type Verity struct {
	// ID is used to correlate `Verity` objects with `FileSystem` objects.
	Id string `yaml:"id" json:"id,omitempty"`
	// The name of the mapper block device.
	// Must be 'root' for the rootfs (/) filesystem.
	Name string `yaml:"name" json:"name,omitempty"`
	// The ID of the 'Partition' to use as the data partition.
	DataDeviceId string `yaml:"dataDeviceId" json:"dataDeviceId,omitempty"`
	// The device ID type used to reference the data partition.
	DataDeviceMountIdType MountIdentifierType `yaml:"dataDeviceMountIdType" json:"dataDeviceMountIdType,omitempty"`
	// The ID of the 'Partition' to use as the hash partition.
	HashDeviceId string `yaml:"hashDeviceId" json:"hashDeviceId,omitempty"`
	// The device ID type used to reference the data partition.
	HashDeviceMountIdType MountIdentifierType `yaml:"hashDeviceMountIdType" json:"hashDeviceMountIdType,omitempty"`
	// How to handle corruption.
	CorruptionOption CorruptionOption `yaml:"corruptionOption" json:"corruptionOption,omitempty"`

	// The filesystem config that points to this verity device.
	// Value is filled in by Storage.IsValid().
	FileSystem *FileSystem
}

func (v *Verity) IsValid() error {
	if v.Id == "" {
		return fmt.Errorf("'id' may not be empty")
	}

	if !verityNameRegex.MatchString(v.Name) {
		return fmt.Errorf("invalid 'name' value (%s)", v.Name)
	}

	if v.DataDeviceId == "" {
		return fmt.Errorf("'dataDeviceId' may not be empty")
	}

	if v.HashDeviceId == "" {
		return fmt.Errorf("'hashDeviceId' may not be empty")
	}

	if err := v.CorruptionOption.IsValid(); err != nil {
		return fmt.Errorf("invalid corruptionOption:\n%w", err)
	}

	return nil
}
