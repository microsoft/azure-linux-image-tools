// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerityIsValid(t *testing.T) {
	validVerity := Verity{
		Id:               "root",
		Name:             "root",
		DataDeviceId:     "root",
		HashDeviceId:     "roothash",
		CorruptionOption: CorruptionOption("panic"),
	}

	err := validVerity.IsValid()
	assert.NoError(t, err)
}

func TestVerityIsValidMissingId(t *testing.T) {
	invalidVerity := Verity{
		Name:         "root",
		DataDeviceId: "root",
		HashDeviceId: "roothash",
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "'id' may not be empty")
}

func TestVerityIsValidInvalidName(t *testing.T) {
	invalidVerity := Verity{
		Id:           "root",
		Name:         "$root",
		DataDeviceId: "root",
		HashDeviceId: "roothash",
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'name' value ($root)")
}

func TestVerityIsValidMissingDataDeviceId(t *testing.T) {
	invalidVerity := Verity{
		Id:           "root",
		Name:         "root",
		HashDeviceId: "roothash",
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "either 'dataDeviceId' or 'dataDevice' must be specified")
}

func TestVerityIsValidMissingHashDeviceId(t *testing.T) {
	invalidVerity := Verity{
		Id:           "root",
		Name:         "root",
		DataDeviceId: "root",
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "either 'hashDeviceId' or 'hashDevice' must be specified")
}

func TestVerityIsValidInvalidCorruptionOption(t *testing.T) {
	invalidVerity := Verity{
		Id:               "root",
		Name:             "root",
		DataDeviceId:     "root",
		HashDeviceId:     "roothash",
		CorruptionOption: CorruptionOption("bad"),
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid CorruptionOption value")
}

func TestVerityIsValidTwoDataDevice(t *testing.T) {
	validVerity := Verity{
		Id:           "root",
		Name:         "root",
		DataDeviceId: "root",
		DataDevice: &IdentifiedPartition{
			IdType: IdentifiedPartitionTypePartLabel,
			Id:     "root",
		},
		HashDeviceId:     "roothash",
		CorruptionOption: CorruptionOption("panic"),
	}

	err := validVerity.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'dataDeviceId' and 'dataDevice'")
}

func TestVerityIsValidTwoHashDevice(t *testing.T) {
	validVerity := Verity{
		Id:           "root",
		Name:         "root",
		DataDeviceId: "root",
		HashDeviceId: "roothash",
		HashDevice: &IdentifiedPartition{
			IdType: IdentifiedPartitionTypePartLabel,
			Id:     "root",
		},
		CorruptionOption: CorruptionOption("panic"),
	}

	err := validVerity.IsValid()
	assert.ErrorContains(t, err, "cannot specify both 'hashDeviceId' and 'hashDevice'")
}

func TestVerityIsValidMismatch(t *testing.T) {
	validVerity := Verity{
		Id:           "root",
		Name:         "root",
		DataDeviceId: "root",
		HashDevice: &IdentifiedPartition{
			IdType: IdentifiedPartitionTypePartLabel,
			Id:     "root",
		},
		CorruptionOption: CorruptionOption("panic"),
	}

	err := validVerity.IsValid()
	assert.ErrorContains(t, err, "cannot use both dataDeviceId/hashDeviceId and dataDevice/hashDevice")
}

func TestVerityIsValidWithValidHashSignaturePath(t *testing.T) {
	validVerity := Verity{
		Id:                "root",
		Name:              "root",
		DataDeviceId:      "root",
		HashDeviceId:      "roothash",
		CorruptionOption:  CorruptionOption("panic"),
		HashSignaturePath: "/boot/root.hash.sig",
	}

	err := validVerity.IsValid()
	assert.NoError(t, err)
}

func TestVerityIsValidWithInvalidHashSignaturePath(t *testing.T) {
	invalidVerity := Verity{
		Id:                "root",
		Name:              "root",
		DataDeviceId:      "root",
		HashDeviceId:      "roothash",
		CorruptionOption:  CorruptionOption("panic"),
		HashSignaturePath: "relative/path.sig",
	}

	err := invalidVerity.IsValid()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid hashSignaturePath")
	assert.ErrorContains(t, err, "must be an absolute path")
}
