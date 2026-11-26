// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVeritySignaturePartition_Empty(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "uuid-1", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found, err := resolveVeritySignaturePartition("", partitions)
	require.NoError(t, err)
	assert.False(t, found, "empty signature path should never resolve to a partition")
}

func TestResolveVeritySignaturePartition_PlainFilePath(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "uuid-1", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found, err := resolveVeritySignaturePartition("/boot/root-hash.sig", partitions)
	require.NoError(t, err)
	assert.False(t, found, "a plain file path should be treated as an embedded file, not a partition")
}

func TestResolveVeritySignaturePartition_Base64Value(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "uuid-1", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found, err := resolveVeritySignaturePartition("base64:QUJDRA==", partitions)
	require.NoError(t, err)
	assert.False(t, found, "an inline base64: value is not partition-backed")
}

func TestResolveVeritySignaturePartition_ByUuid(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found, err := resolveVeritySignaturePartition("/dev/disk/by-uuid/cccc-dddd", partitions)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 2, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_ByPartUuid(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found, err := resolveVeritySignaturePartition("/dev/disk/by-partuuid/partuuid-2", partitions)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 2, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_ByPartLabel(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "root-hash-sig"},
		{PartitionNum: 2, Uuid: "cccc-dddd", PartUuid: "partuuid-2", PartLabel: "label-2"},
	}

	match, found, err := resolveVeritySignaturePartition("/dev/disk/by-partlabel/root-hash-sig", partitions)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_ByPartLabelEscaped(t *testing.T) {
	// udev escapes bytes unsafe for a symlink name (e.g. spaces) as `\xHH`
	// when constructing /dev/disk/by-partlabel/<label>.
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "root hash sig"},
	}

	match, found, err := resolveVeritySignaturePartition(`/dev/disk/by-partlabel/root\x20hash\x20sig`, partitions)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1, match.PartitionNum)
}

func TestResolveVeritySignaturePartition_NoMatch(t *testing.T) {
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}
	for _, path := range []string{
		"/dev/disk/by-uuid/does-not-exist",
		"/dev/disk/by-partuuid/does-not-exist",
		"/dev/disk/by-partlabel/does-not-exist",
	} {
		t.Run(path, func(t *testing.T) {
			match, found, err := resolveVeritySignaturePartition(path, partitions)
			require.ErrorContains(t, err, path)
			assert.False(t, found)
			assert.Equal(t, outputPartitionMetadata{}, match)
		})
	}
}

func TestResolveVeritySignaturePartition_FstabStyleNotSupported(t *testing.T) {
	// The fstab-style UUID=/PARTUUID=/PARTLABEL= syntax used for
	// systemd.verity_root_data=/_hash= is NOT what systemd's veritysetup
	// parser accepts for root-hash-signature=, so it must not resolve to a
	// partition even if it happens to match one by coincidence.
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "label-1"},
	}

	_, found, err := resolveVeritySignaturePartition("PARTUUID=partuuid-1", partitions)
	require.NoError(t, err)
	assert.False(t, found, "fstab-style PARTUUID= syntax is not a real root-hash-signature= value")
}

func TestResolveVeritySignaturePartition_ByLabelNotSupported(t *testing.T) {
	// /dev/disk/by-label (filesystem label) is not one of the forms
	// root-hash-signature= can reference for a raw partition.
	partitions := []outputPartitionMetadata{
		{PartitionNum: 1, Uuid: "aaaa-bbbb", PartUuid: "partuuid-1", PartLabel: "root-hash-sig"},
	}

	_, found, err := resolveVeritySignaturePartition("/dev/disk/by-label/root-hash-sig", partitions)
	require.NoError(t, err)
	assert.False(t, found, "/dev/disk/by-label should not be treated as a partition identifier here")
}
