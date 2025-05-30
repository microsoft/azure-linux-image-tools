// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/ptrutils"
	"github.com/stretchr/testify/assert"
)

func TestPartitionIsValidExpanding(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestPartitionIsValidFixedSize(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestPartitionIsValidZeroSize(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   ptrutils.PtrTo(DiskSize(0)),
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "partition")
	assert.ErrorContains(t, err, "size")
}

func TestPartitionIsValidZeroSizeV2(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		Size: PartitionSize{
			Type: PartitionSizeTypeExplicit,
			Size: 0,
		},
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "size can't be 0 or negative")
}

func TestPartitionIsValidNegativeSize(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
		End:   ptrutils.PtrTo(DiskSize(1 * diskutils.MiB)),
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "size can't be 0 or negative")
}

func TestPartitionIsValidBothEndAndSize(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
		End:   ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
		Size: PartitionSize{
			Type: PartitionSizeTypeExplicit,
			Size: 1 * diskutils.MiB,
		},
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "cannot specify both end and size on partition")
}

func TestPartitionIsValidEndAndGrow(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(2 * diskutils.MiB)),
		End:   ptrutils.PtrTo(DiskSize(3 * diskutils.MiB)),
		Size: PartitionSize{
			Type: PartitionSizeTypeGrow,
		},
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "cannot specify both end and size on partition")
}

func TestPartitionIsValidGoodName(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Label: "a",
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestPartitionIsValidNameTooLong(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Label: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "name")
	assert.ErrorContains(t, err, "too long")
}

func TestPartitionIsValidNameNonASCII(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Label: "❤️",
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "name")
	assert.ErrorContains(t, err, "ASCII")
}

func TestPartitionIsValidGoodType(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Type:  PartitionTypeESP,
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestPartitionIsValidBadType(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Type:  PartitionType("a"),
	}

	err := partition.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "partition type is unknown and is not a UUID (a)")
}

func TestPartitionIsValidTypeUuid(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Type:  "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
	}

	err := partition.IsValid()
	assert.NoError(t, err)
}

func TestPartitionIsValidTypeUuidInvalid(t *testing.T) {
	partition := Partition{
		Id:    "a",
		Start: ptrutils.PtrTo(DiskSize(0)),
		End:   nil,
		Type:  "c12a7328-f81f-11d2-ba4b-00a0c93ec93",
	}

	err := partition.IsValid()
	assert.ErrorContains(t, err, "partition type is unknown and is not a UUID (c12a7328-f81f-11d2-ba4b-00a0c93ec93)")
}
