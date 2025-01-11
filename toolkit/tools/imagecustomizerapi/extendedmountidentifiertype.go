// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import "fmt"

// ExtendedMountIdentifierType indicates how a partition should be identified in the fstab file.
type ExtendedMountIdentifierType string

const (
	// ExtendedMountIdentifierTypeUuid mounts this partition via the filesystem UUID.
	ExtendedMountIdentifierTypeUuid ExtendedMountIdentifierType = "uuid"

	// ExtendedMountIdentifierTypePartUuid mounts this partition via the GPT/MBR PARTUUID.
	ExtendedMountIdentifierTypePartUuid ExtendedMountIdentifierType = "part-uuid"

	// ExtendedMountIdentifierTypePartLabel mounts this partition via the GPT PARTLABEL.
	ExtendedMountIdentifierTypePartLabel ExtendedMountIdentifierType = "part-label"

	// ExtendedMountIdentifierTypeDev mounts this partition via a device.
	ExtendedMountIdentifierTypeDev ExtendedMountIdentifierType = "dev"

	// ExtendedMountIdentifierTypeDefault uses the default type, which is PARTUUID.
	ExtendedMountIdentifierTypeDefault ExtendedMountIdentifierType = ""
)

func (e ExtendedMountIdentifierType) IsValid() error {
	switch e {
	case ExtendedMountIdentifierTypeUuid, ExtendedMountIdentifierTypePartUuid, ExtendedMountIdentifierTypePartLabel, ExtendedMountIdentifierTypeDev, ExtendedMountIdentifierTypeDefault:
		// All good.
		return nil

	default:
		return fmt.Errorf("invalid value (%v)", e)
	}
}
