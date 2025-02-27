// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentifiedPartitionIsValidValidPartLabel(t *testing.T) {
	validPartition := IdentifiedPartition{
		IdType: "part-label",
		Id:     "ValidLabelName",
	}

	err := validPartition.IsValid()
	assert.NoError(t, err)
}

func TestIdentifiedPartitionIsValidEmptyPartLabel(t *testing.T) {
	invalidPartition := IdentifiedPartition{
		IdType: "part-label",
		Id:     "",
	}

	err := invalidPartition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid id: empty string")
}

func TestIdentifiedPartitionIsValidInvalidIdType(t *testing.T) {
	incorrectUuidPartition := IdentifiedPartition{
		IdType: "cat",
		Id:     "incorrect-uuid-format",
	}

	err := incorrectUuidPartition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid idType")
	assert.ErrorContains(t, err, "invalid value (cat)")
}

func TestIdentifiedPartitionIsValidInvalidPartLabel(t *testing.T) {
	incorrectUuidPartition := IdentifiedPartition{
		IdType: IdentifiedPartitionTypePartLabel,
		Id:     "i ❤️ cats",
	}

	err := incorrectUuidPartition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid id format for part-label")
	assert.ErrorContains(t, err, "partition name (i ❤️ cats) contains a non-ASCII character (❤)")
}
