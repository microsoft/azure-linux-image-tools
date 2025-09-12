// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package osmodifierapi

import (
	"fmt"
	"regexp"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
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
	// The 'Partition' to use as the data partition.
	DataDevice string `yaml:"dataDevice"`
	// The 'Partition' to use as the hash partition.
	HashDevice string `yaml:"hashDevice"`
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

	if v.DataDevice == "" {
		return fmt.Errorf("'dataDevice' may not be empty")
	}

	if v.HashDevice == "" {
		return fmt.Errorf("'hashDevice' may not be empty")
	}

	if err := v.CorruptionOption.IsValid(); err != nil {
		return fmt.Errorf("invalid corruptionOption:\n%w", err)
	}

	return nil
}
