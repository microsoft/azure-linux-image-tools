// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

// MountIdentifier indicates how a partition should be identified in the fstab file
type MountIdentifier string

// label
const (
	// MountIdentifierUuid mounts this partition via the filesystem UUID
	MountIdentifierUuid MountIdentifier = "uuid"
	// MountIdentifierPartUuid mounts this partition via the GPT/MBR PARTUUID
	MountIdentifierPartUuid MountIdentifier = "partuuid"
	// MountIdentifierPartLabel mounts this partition via the GPT PARTLABEL
	MountIdentifierPartLabel MountIdentifier = "partlabel"

	// There is not a clear way to set arbitrary partitions via a device path (ie /dev/sda1)
	// so we do not support those.

	// We currently do not set filesystem LABELS, so those are also not useful here.

	MountIdentifierDefault MountIdentifier = MountIdentifierPartUuid
	MountIdentifierNone    MountIdentifier = ""
)
