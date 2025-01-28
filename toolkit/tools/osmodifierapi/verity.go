// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierapi

import (
	"fmt"
	"regexp"

	"github.com/microsoft/azurelinux/toolkit/tools/imagecustomizerapi"
)

var (
	verityNameRegex = regexp.MustCompile("^[a-z]+$")
)

type Verity struct {
	// ID is used to correlate `Verity` objects with `FileSystem` objects.
	Id string `yaml:"id"`
	// The name of the mapper block device.
	// Must be 'root' for the rootfs (/) filesystem.
	Name string `yaml:"name"`
	// The ID of the 'Partition' to use as the data partition.
	DataDeviceId string `yaml:"dataDeviceId"`
	// The ID of the 'Partition' to use as the hash partition.
	HashDeviceId string `yaml:"hashDeviceId"`
	// How to handle corruption.
	CorruptionOption imagecustomizerapi.CorruptionOption `yaml:"corruptionOption"`
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
