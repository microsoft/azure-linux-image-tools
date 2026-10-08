// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVeritySignaturePartition_Empty(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "uuid-1", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found := resolveVeritySignaturePartition("", partitions)
	assert.False(t, found, "empty signature path should never resolve to a partition")
}

func TestResolveVeritySignaturePartition_FilePath(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "uuid-1", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found := resolveVeritySignaturePartition("/boot/root-hash.sig", partitions)
	assert.False(t, found, "a plain file path should be treated as an embedded file, not a partition")
}

func TestResolveVeritySignaturePartition_ByUuid(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found := resolveVeritySignaturePartition("UUID=cccc-dddd", partitions)
	assert.True(t, found)
	assert.Equal(t, 2, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_ByPartUuid(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found := resolveVeritySignaturePartition("PARTUUID=partuuid-2", partitions)
	assert.True(t, found)
	assert.Equal(t, 2, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_ByPartLabel(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "root-hash-sig"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found := resolveVeritySignaturePartition("PARTLABEL=root-hash-sig", partitions)
	assert.True(t, found)
	assert.Equal(t, 1, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_NoMatch(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found := resolveVeritySignaturePartition("UUID=does-not-exist", partitions)
	assert.False(t, found, "an identifier that matches no partition should not resolve")
}

func TestResolveVeritySignaturePartition_LabelNotSupported(t *testing.T) {
	// LABEL= (filesystem label) is a valid fstab source type in general, but
	// is not one of the partition-identifying formats this resolver accepts
	// for a signature partition (UUID=, PARTUUID=, PARTLABEL=), so it must be
	// treated as "not a partition" rather than matched against PartLabel.
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "root-hash-sig"},
	}

	_, found := resolveVeritySignaturePartition("LABEL=root-hash-sig", partitions)
	assert.False(t, found, "LABEL= should not be treated as a partition identifier here")
}
