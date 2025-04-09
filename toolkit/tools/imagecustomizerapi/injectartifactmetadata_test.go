// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectArtifactMetadata_Valid(t *testing.T) {
	entry := InjectArtifactMetadata{
		Source:         "./bootx64.signed.efi",
		Destination:    "/EFI/BOOT/bootx64.efi",
		UnsignedSource: "./bootx64.efi",
		Partition: InjectFilePartition{
			MountIdType: MountIdentifierTypePartUuid,
			Id:          "5678-EFGH",
		},
	}

	err := entry.IsValid()
	assert.NoError(t, err)
}

func TestInjectArtifactMetadata_MissingSource(t *testing.T) {
	entry := InjectArtifactMetadata{
		Source:         "",
		Destination:    "/EFI/BOOT/bootx64.efi",
		UnsignedSource: "./bootx64.efi",
		Partition: InjectFilePartition{
			MountIdType: MountIdentifierTypePartUuid,
			Id:          "5678-EFGH",
		},
	}

	err := entry.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source or destination is empty")
}

func TestInjectArtifactMetadata_InvalidPartition(t *testing.T) {
	entry := InjectArtifactMetadata{
		Source:         "./bootx64.signed.efi",
		Destination:    "/EFI/BOOT/bootx64.efi",
		UnsignedSource: "./bootx64.efi",
		Partition: InjectFilePartition{
			MountIdType: "bad-type",
			Id:          "5678-EFGH",
		},
	}

	err := entry.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid partition")
}
