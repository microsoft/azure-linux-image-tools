// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import "github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"

const (
	// Internal use only:
	// Used for reading Ubuntu images, which use filesystem label to identify partitions
	// in the fstab file.
	// Future: Move this value into MountIdentifierType when support for filesystem labels
	// are added to the API.
	MountIdentifierTypeLabel imagecustomizerapi.MountIdentifierType = "label"

	// Internal use only:
	// Used for handling verity device mappings (e.g., /dev/mapper/root).
	MountIdentifierTypeDev imagecustomizerapi.MountIdentifierType = "dev"
)
