// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagegen/diskutils"
	"github.com/stretchr/testify/assert"
)

func TestPartitionSizeGrow(t *testing.T) {
	var size PartitionSize
	err := UnmarshalAndValidateYaml([]byte("grow"), &size)
	assert.NoError(t, err)
}

func TestPartitionSizeMiB(t *testing.T) {
	var size PartitionSize
	err := UnmarshalAndValidateYaml([]byte("1M"), &size)
	assert.NoError(t, err)
	assert.Equal(t, PartitionSize{PartitionSizeTypeExplicit, 1 * diskutils.MiB}, size)
}

func TestPartitionSizeInvalidNotString(t *testing.T) {
	var size PartitionSize
	err := UnmarshalAndValidateYaml([]byte("[]"), &size)
	assert.ErrorContains(t, err, "failed to parse partition size")
}

func TestPartitionSizeInvalidValue(t *testing.T) {
	var size PartitionSize
	err := UnmarshalAndValidateYaml([]byte("cat"), &size)
	assert.ErrorContains(t, err, "(cat) has incorrect format")
}

func TestPartitionSizeInvalidType(t *testing.T) {
	size := PartitionSize{
		Type: -1,
	}
	err := size.IsValid()
	assert.ErrorContains(t, err, "invalid partition size type (-1)")
}
