// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package imagecustomizerlib

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/imagecustomizerapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A representative ACL sfdisk --dump: ESP, USR-A/B (with A/B attr bits), OEM, ROOT.
const aclSampleDump = `label: gpt
label-id: 7A376709-9E8D-4F83-AF56-C9CFB3FFBE93
device: /dev/loop0
unit: sectors
first-lba: 2048
last-lba: 409566
sector-size: 512

/dev/loop0p1 : start=        2048, size=      524288, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=96EC7BA6-E683-4E06-B402-40F644418B49, name="EFI-SYSTEM"
/dev/loop0p2 : start=      526336, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=91BCEEAE-BA93-4CAE-BD12-11694ED8B8C5, name="USR-A", attrs="GUID:48,56"
/dev/loop0p3 : start=     2623488, size=     2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, uuid=0241E9F2-CF5A-485C-AEAF-6A1FB19094E8, name="USR-B", attrs="GUID:48,56"
/dev/loop0p4 : start=     4720640, size=      262144, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=2458B054-19AB-4EFD-8738-B04148A3B2CC, name="OEM"
/dev/loop0p5 : start=     4982784, size=    58720256, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=17CA28A9-E145-48C6-BC2D-7D7D125804CE, name="ROOT"`

func parseAclSample(t *testing.T) *aclPartitionTable {
	table, err := parseAclPartitionTable(aclSampleDump)
	require.NoError(t, err)
	return table
}

func TestParseAclPartitionTable(t *testing.T) {
	table := parseAclSample(t)

	assert.Equal(t, "7A376709-9E8D-4F83-AF56-C9CFB3FFBE93", table.labelId)
	assert.Equal(t, uint64(2048), table.firstLba)
	assert.Equal(t, uint64(512), table.sectorSize)
	require.Len(t, table.partitions, 5)

	assert.Equal(t, aclPartLabelEsp, table.partitions[0].label)
	assert.Equal(t, aclPartLabelUsrA, table.partitions[1].label)
	assert.Equal(t, aclPartLabelUsrB, table.partitions[2].label)
	assert.Equal(t, aclPartLabelOem, table.partitions[3].label)
	assert.Equal(t, aclPartLabelRoot, table.partitions[4].label)

	// Sizes/starts parsed.
	assert.Equal(t, uint64(2048), table.partitions[0].startSect)
	assert.Equal(t, uint64(2097152), table.partitions[1].sizeSect)
}

func TestParseAclPartitionLinePreservesAttrs(t *testing.T) {
	entry, err := parseAclPartitionLine(
		`/dev/loop0p2 : start=526336, size=2097152, type=5DFBF5F4-2848-4BAC-AA5E-0D9A20B745A6, ` +
			`uuid=91BCEEAE-BA93-4CAE-BD12-11694ED8B8C5, name="USR-A", attrs="GUID:48,56"`)
	require.NoError(t, err)

	// The attrs value contains a comma and must round-trip intact.
	var attrs string
	for _, f := range entry.fields {
		if f.key == "attrs" {
			attrs = f.value
		}
	}
	assert.Equal(t, `"GUID:48,56"`, attrs)
}

func TestResolveAclRequestedSizesGrowOnly(t *testing.T) {
	table := parseAclSample(t)

	// USR-A/B currently 2097152 sectors * 512 = 1 GiB. Request 2 GiB: grow.
	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: 2 * 1024 * 1024 * 1024},
	}
	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)
	assert.Equal(t, uint64(2*1024*1024*1024), sizes[aclPartLabelUsrA])
	assert.Equal(t, uint64(2*1024*1024*1024), sizes[aclPartLabelUsrB])
}

func TestResolveAclRequestedSizesShrinkRejected(t *testing.T) {
	table := parseAclSample(t)

	// Request a smaller /usr: must error.
	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: 512 * 1024 * 1024},
	}
	_, err := resolveAclRequestedSizes(acl, table)
	require.Error(t, err)
	assert.ErrorContains(t, err, "grow-only")
}

func TestResolveAclRequestedSizesNoOp(t *testing.T) {
	table := parseAclSample(t)

	// Request the exact current size: no-op (empty result).
	acl := &imagecustomizerapi.Acl{
		Usr: &imagecustomizerapi.AclPartitionGrow{Size: 1024 * 1024 * 1024},
	}
	sizes, err := resolveAclRequestedSizes(acl, table)
	require.NoError(t, err)
	assert.Empty(t, sizes)
}

func TestApplyAclGrownLayoutUsr(t *testing.T) {
	table := parseAclSample(t)

	// Capture ROOT's original size before growing.
	origRootSizeSect := table.partitions[4].sizeSect

	requestedSizes := map[string]uint64{
		aclPartLabelUsrA: 2 * 1024 * 1024 * 1024,
		aclPartLabelUsrB: 2 * 1024 * 1024 * 1024,
	}
	growthBytes, espRecreated := applyAclGrownLayout(table, requestedSizes)
	assert.False(t, espRecreated)

	// Growth is the sum of the per-partition deltas: 2 x (2 GiB - 1 GiB) = 2 GiB.
	assert.Equal(t, uint64(2*1024*1024*1024), growthBytes)

	// USR-A and USR-B grown to 2 GiB (4194304 sectors).
	assert.Equal(t, uint64(4194304), table.partitions[1].sizeSect)
	assert.Equal(t, uint64(4194304), table.partitions[2].sizeSect)

	// Partitions remain contiguous: each start == previous start + previous size.
	for i := 1; i < len(table.partitions); i++ {
		prev := table.partitions[i-1]
		assert.Equal(t, prev.startSect+prev.sizeSect, table.partitions[i].startSect,
			"partition %d start is not contiguous", i)
	}

	// ROOT (last) keeps its original size and is merely shifted right (never shrunk).
	assert.Equal(t, origRootSizeSect, table.partitions[4].sizeSect,
		"ROOT should keep its original size")
	hasSize := false
	for _, f := range table.partitions[4].fields {
		if f.key == "size" {
			hasSize = true
			assert.Equal(t, strconv.FormatUint(origRootSizeSect, 10), f.value)
		}
	}
	assert.True(t, hasSize, "ROOT should keep an explicit size")
}

func TestApplyAclGrownLayoutEsp(t *testing.T) {
	table := parseAclSample(t)

	origRootSizeSect := table.partitions[4].sizeSect

	requestedSizes := map[string]uint64{
		aclPartLabelEsp: 512 * 1024 * 1024,
	}
	growthBytes, espRecreated := applyAclGrownLayout(table, requestedSizes)
	assert.True(t, espRecreated)
	assert.Equal(t, uint64(512*1024*1024/512), table.partitions[0].sizeSect)

	// Growth is ESP delta: 512 MiB - 256 MiB = 256 MiB.
	assert.Equal(t, uint64(256*1024*1024), growthBytes)

	// Everything after ESP shifts right and stays contiguous.
	for i := 1; i < len(table.partitions); i++ {
		prev := table.partitions[i-1]
		assert.Equal(t, prev.startSect+prev.sizeSect, table.partitions[i].startSect)
	}

	// ROOT keeps its original size.
	assert.Equal(t, origRootSizeSect, table.partitions[4].sizeSect)
}

func TestAclAlignedDiskSizeGrowsByDelta(t *testing.T) {
	// A base file whose size is a whole MiB; growth of 2 GiB yields base + 2 GiB.
	tmpFile := filepath.Join(t.TempDir(), "base.raw")
	const baseBytes = uint64(30 * 1024 * 1024 * 1024)
	require.NoError(t, os.WriteFile(tmpFile, nil, 0o644))
	require.NoError(t, os.Truncate(tmpFile, int64(baseBytes)))

	growth := uint64(2 * 1024 * 1024 * 1024)
	got := aclAlignedDiskSize(tmpFile, growth)
	assert.Equal(t, baseBytes+growth, got)
}

func TestRestoreAclPartitionTableRoundTrip(t *testing.T) {
	table := parseAclSample(t)
	applyAclGrownLayout(table, map[string]uint64{
		aclPartLabelUsrA: 2 * 1024 * 1024 * 1024,
		aclPartLabelUsrB: 2 * 1024 * 1024 * 1024,
	})

	script := buildAclSfdiskScript(table)

	// Header + identity preserved.
	assert.Contains(t, script, "label: gpt")
	assert.Contains(t, script, "label-id: 7A376709-9E8D-4F83-AF56-C9CFB3FFBE93")
	assert.Contains(t, script, "first-lba: 2048")
	// A/B attr bits preserved for both USR partitions.
	assert.Equal(t, 2, strings.Count(script, `attrs="GUID:48,56"`))
	// PARTUUIDs preserved.
	assert.Contains(t, script, "uuid=91BCEEAE-BA93-4CAE-BD12-11694ED8B8C5")
	assert.Contains(t, script, "uuid=17CA28A9-E145-48C6-BC2D-7D7D125804CE")
	// ROOT keeps an explicit size (it is not filled/shrunk).
	assert.Contains(t, script, "size=58720256")
}

func TestParseAclPartitionTableRejectsEmpty(t *testing.T) {
	_, err := parseAclPartitionTable("label: gpt\nunit: sectors\n")
	assert.Error(t, err)
}
