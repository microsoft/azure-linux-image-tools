// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResizePartitionRefUnmarshalLast(t *testing.T) {
	text := "last"

	expected := ResizePartitionRef{
		IdType: ResizePartitionRefTypeLast,
	}

	var value ResizePartitionRef
	err := UnmarshalYaml([]byte(text), &value)
	assert.NoError(t, err)
	assert.Equal(t, expected, value)
	assert.NoError(t, value.IsValid())
}

func TestResizePartitionRefUnmarshalInvalidString(t *testing.T) {
	text := "sample"
	var value ResizePartitionRef
	err := UnmarshalYaml([]byte(text), &value)
	assert.ErrorContains(t, err, "invalid ResizePartitionRef value (sample)")
}

func TestResizePartitionRefUnmarshalStructLast(t *testing.T) {
	text := "{ \"idType\":\"last\" }"

	expected := ResizePartitionRef{
		IdType: ResizePartitionRefTypeLast,
	}

	var value ResizePartitionRef
	err := UnmarshalYaml([]byte(text), &value)
	assert.NoError(t, err)
	assert.Equal(t, expected, value)
	assert.NoError(t, value.IsValid())
}

func TestResizePartitionRefUnmarshalStructPartUuid(t *testing.T) {
	text := "{ \"idType\":\"label\", \"id\":\"sample\" }"

	expected := ResizePartitionRef{
		IdType: ResizePartitionRefTypeLabel,
		Id:     "sample",
	}

	var value ResizePartitionRef
	err := UnmarshalYaml([]byte(text), &value)
	assert.NoError(t, err)
	assert.Equal(t, expected, value)
	assert.NoError(t, value.IsValid())
}

func TestResizePartitionRefUnmarshalBadField(t *testing.T) {
	text := "{ \"bad\":\"value\" }"

	var value ResizePartitionRef
	err := UnmarshalYaml([]byte(text), &value)
	assert.ErrorContains(t, err, "bad")
}

func TestResizePartitionIsValidLastHasId(t *testing.T) {
	value := ResizePartitionRef{
		IdType: ResizePartitionRefTypeLast,
		Id:     "hello",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "'id' value must be empty for 'idType' of 'last'")
}

func TestResizePartitionIsValidLabelDoesNotHaveId(t *testing.T) {
	value := ResizePartitionRef{
		IdType: ResizePartitionRefTypeLabel,
		Id:     "",
	}
	err := value.IsValid()
	assert.ErrorContains(t, err, "'id' value must not be empty for 'idType' of 'label'")
}
