// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

// ExtendedMountIdentifierType indicates how a partition should be identified in the fstab file.
// This type was introduced to extend the functionality of MountIdentifierType while preserving
// the original public API structure. MountIdentifierType is part of a public API and cannot be
// modified to include new identifiers without breaking backward compatibility.
// ExtendedMountIdentifierType provides additional flexibility for internal use without affecting the public API.
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

	// ToDo: overlay, disk path mount id type for LG to customize overlay enabled base image.
	//
	ExtendedMountIdentifierTypeOverlay  ExtendedMountIdentifierType = "overlay"
	ExtendedMountIdentifierTypeDiskPath ExtendedMountIdentifierType = "disk-path"

	// ExtendedMountIdentifierTypeDefault uses the default type, which is PARTUUID.
	ExtendedMountIdentifierTypeDefault ExtendedMountIdentifierType = ""
)
