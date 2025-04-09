// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectFilePartition_Valid(t *testing.T) {
	partition := InjectFilePartition{
		MountIdType: MountIdentifierTypePartUuid,
		Id:          "1234-ABCD",
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestInjectFilePartition_EmptyId(t *testing.T) {
	partition := InjectFilePartition{
		MountIdType: MountIdentifierTypePartUuid,
		Id:          "",
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "partition id is empty")
}

func TestInjectFilePartition_InvalidMountIdType(t *testing.T) {
	partition := InjectFilePartition{
		MountIdType: "bad-type",
		Id:          "1234-ABCD",
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount id type")
}
